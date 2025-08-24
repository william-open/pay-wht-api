package service

import (
	"errors"
	"fmt"
	"github.com/jinzhu/copier"
	"github.com/shopspring/decimal"
	"log"
	"strconv"
	"time"
	mainmodel "wht-order-api/internal/model/main"
	"wht-order-api/internal/utils"

	"wht-order-api/internal/dal"
	"wht-order-api/internal/dao"
	"wht-order-api/internal/dto"
	"wht-order-api/internal/idgen"
	ordermodel "wht-order-api/internal/model/order"
)

type PayoutOrderService struct {
	mainDao       *dao.MainDao
	orderDao      *dao.PayoutOrderDao
	indexTableDao *dao.IndexTableDao
}

func NewPayoutOrderService() *PayoutOrderService {
	return &PayoutOrderService{
		mainDao:       &dao.MainDao{},
		orderDao:      &dao.PayoutOrderDao{},
		indexTableDao: &dao.IndexTableDao{},
	}
}

// Create 处理代收订单下单业务逻辑
func (s *PayoutOrderService) Create(req dto.CreatePayoutOrderReq) (dto.CreatePayoutOrderResp, error) {
	var response dto.CreatePayoutOrderResp
	// 1) 主库校验
	merchant, err := s.mainDao.GetMerchant(req.MerchantNo)
	if err != nil || merchant == nil || merchant.Status != 1 {
		return response, errors.New("merchant invalid")
	}
	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		return response, errors.New("金额格式错误")
	}
	// 查询系统通道是否有效
	channelDetail, err := s.QuerySysChannel(req.PayType)
	if err != nil {
		return response, errors.New("通道不存在")
	}
	log.Printf("代付请求通道编码: %v", req.PayType)
	// 1) 选择可用上游通道
	channel, err := s.SelectPaymentChannel(uint(merchant.MerchantID), amount, req.PayType, channelDetail.Currency)
	if err != nil || channel.Coding == "" {
		log.Printf("代付请求，没有可用通道: %v", req.PayType)
		return response, errors.New("没有可用通道")
	}

	now := time.Now()
	// 2) 全局订单ID
	oid := idgen.New()
	table := utils.GetShardOrderTable("p_out_order", oid, now)
	log.Printf("代付，插入数据表: %+v", table)

	// 3) 幂等校验（建议用唯一索引保障）
	if exist, _ := s.orderDao.GetByMerchantNo(table, merchant.MerchantID, req.TranFlow); exist != nil {
		return response, nil
	}

	//var upRespData dto.UpstreamResponse
	//if err := json.Unmarshal([]byte(upResp), &upRespData); err != nil {
	//	fmt.Println("解析失败:", err)
	//	return 0, errors.New("请求上游供应商,解析失败")
	//}
	//upRespData.RawResponse = upResp // 保留原始响应，便于日志追踪

	// 5) 订单结算计算
	var agentPct, agentFixed = decimal.Zero, decimal.Zero
	if merchant.PId > 0 {
		var agentMerchant dto.QueryAgentMerchant
		agentMerchant.AId = int64(merchant.PId)
		agentMerchant.MId = int64(merchant.MerchantID)
		agentMerchant.SysChannelID = channel.SysChannelID
		agentMerchant.UpChannelID = channel.UpChannelId
		agentMerchant.Currency = channel.Currency
		agentInfo, err := s.mainDao.GetAgentMerchant(agentMerchant)
		if err != nil && agentInfo != nil && agentInfo.Status == 1 {
			agentPct = agentInfo.DefaultRate
			agentFixed = agentInfo.SingleFee
		}
	}
	// 订单金额转化浮点数
	orderAmount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		return response, errors.New("无效的浮点数")
	}
	settle := utils.Calculate(orderAmount, channel.MDefaultRate, channel.MSingleFee, agentPct, agentFixed, channel.UpDefaultRate, channel.UpSingleFee, "agent_from_platform", channel.Currency)
	var orderSettle dto.SettlementResult
	_ = copier.Copy(&orderSettle, &settle)

	// 6) 构造订单模型
	m := &ordermodel.MerchantPayOutOrderM{
		OrderID:        oid,
		MID:            merchant.MerchantID,
		MOrderID:       req.TranFlow,
		Amount:         orderAmount,
		Currency:       channel.Currency,
		SupplierID:     channel.UpstreamId,
		Status:         1,
		NotifyURL:      req.NotifyUrl,
		ChannelID:      channel.SysChannelID,
		UpChannelID:    channel.UpChannelId,
		ChannelCode:    &channel.Coding,
		PayEmail:       req.PayEmail,
		PayPhone:       req.PayPhone,
		PayMethod:      req.PayMethod,
		BankCode:       req.BankCode,
		BankName:       req.BankName,
		IdentityNum:    req.IdentityNum,
		IdentityType:   req.IdentityType,
		AccountName:    req.AccName,
		AccountNo:      req.AccNo,
		MTitle:         &merchant.NickName,
		ChannelTitle:   &channel.SysChannelTitle,
		UpChannelCode:  &channel.UpstreamCode,
		UpChannelTitle: &channel.UpChannelTitle,
		MRate:          &channel.MDefaultRate,
		UpRate:         &channel.UpChannelRate,
		MFixedFee:      &channel.MSingleFee,
		UpFixedFee:     &channel.UpSingleFee,
		Fees:           settle.MerchantTotalFee,
		Country:        &channel.Country,
		SettleSnapshot: ordermodel.PayoutSettleSnapshot(orderSettle),
	}

	// 7) 插入数据库
	if err := s.orderDao.Insert(table, m); err != nil {
		return response, err
	}

	txId := idgen.New()
	txTable := utils.GetShardOrderTable("p_up_out_order", txId, time.Now())

	// 8) 生成代付分表索引表
	payoutIndexTable := utils.GetOrderIndexTable("p_out_order_index", time.Now())
	orderIndexTable := utils.GetShardOrderTable("p_out_order_log", txId, time.Now())
	payoutIndex := &ordermodel.PayoutOrderIndexM{
		MID:               merchant.MerchantID,
		MOrderID:          req.TranFlow,
		OrderID:           oid,
		OrderTableName:    payoutIndexTable,
		OrderLogTableName: orderIndexTable,
	}
	if err := s.orderDao.InsertPayoutOrderIndexTable(payoutIndexTable, payoutIndex); err != nil {
		return response, err
	}

	// 9. 写入上游流水
	tx := &ordermodel.PayoutUpstreamTxM{
		OrderID:    oid,
		MerchantID: strconv.FormatUint(merchant.MerchantID, 10),
		SupplierId: uint64(channel.UpstreamId),
		Amount:     req.Amount,
		Currency:   channel.Currency,
		Status:     0,
		UpOrderId:  txId,
		CreateTime: time.Now(),
	}

	// 7) 插入上游交易数据库
	if err := s.orderDao.InsertTx(txTable, tx); err != nil {
		return response, err
	}
	// 更新订单表
	var updateOrder dto.UpdateOrderVo
	updateOrder.OrderId = oid
	updateOrder.UpOrderId = txId
	updateOrder.UpdateTime = time.Now()
	if err := s.orderDao.UpdateOrder(table, updateOrder); err != nil {
		return response, err
	}
	// 4) 调用上游下单接口生成支付链接

	var upstreamRequest dto.UpstreamRequest
	upstreamRequest.Currency = channel.Currency
	upstreamRequest.Amount = req.Amount
	upstreamRequest.PayType = req.PayType
	upstreamRequest.Currency = channel.Currency
	upstreamRequest.ProviderKey = channel.ChannelCode
	upstreamRequest.MchOrderId = strconv.FormatUint(txId, 10)
	upstreamRequest.ApiKey = merchant.ApiKey
	upstreamRequest.MchNo = channel.UpAccount
	upstreamRequest.NotifyUrl = req.NotifyUrl
	upstreamRequest.IdentityType = req.IdentityType
	upstreamRequest.IdentityNum = req.IdentityNum
	upstreamRequest.PayMethod = req.PayMethod

	mOrderId, upOrderNo, _, err := CallUpstreamService(upstreamRequest, channel)
	if err != nil {
		fmt.Println("请求上游供应商失败:", err.Error())
		return response, errors.New("请求上游供应商失败")
	}
	// 更新上游交易订单信息
	var upTx dto.UpdateUpTxVo

	var mOrderIdUint uint64
	if mOrderId != "" {
		mOrderIdUint, err = strconv.ParseUint(mOrderId, 10, 64)
		if err != nil {
			log.Printf("上游订单号转换失败: %v", err)
			return response, errors.New("上游订单号转换失败")
		}
	}

	upTx.UpOrderId = mOrderIdUint
	upTx.UpOrderNo = upOrderNo
	if err := s.orderDao.UpdateUpTx(txTable, upTx); err != nil {
		return response, err
	}

	response.PaySerialNo = string(oid) //订单表编号
	response.TranFlow = req.TranFlow
	response.SysTime = time.Now().String()
	response.Amount = req.Amount

	// 9) 缓存到 Redis
	cacheKey := "payout_order:" + strconv.FormatUint(oid, 10)
	_ = dal.RedisClient.Set(dal.RedisCtx, cacheKey, utils.MapToJSON(m), 10*time.Minute).Err()

	// 10) 发布 MQ 事件
	//evt := mq.OrderCreatedEvent{
	//	OrderID: oid, MerchantID: merchant.MerchantID, MerchantOrdNo: req.TranFlow,
	//	Amount: req.Amount, Currency: req.Currency, PayMethod: "", CreatedAt: now.Unix(),
	//}
	//_ = mq.PublishOrderCreated(evt)
	response.Code = string(rune(0))
	return response, nil
}

func (s *PayoutOrderService) Get(param dto.QueryPayoutOrderReq) (dto.QueryPayoutOrderResp, error) {
	var resp dto.QueryPayoutOrderResp

	indexTable := utils.GetOrderIndexTable("p_out_order_index", time.Now())

	mId, err := s.GetMerchantInfo(param.MerchantNo)
	if err != nil {
		return resp, err
	}

	indexTableResult, _ := s.indexTableDao.GetByIndexTable(indexTable, param.TranFlow, mId)
	orderIndexTable := utils.GetShardOrderTable("p_out_order", indexTableResult.OrderID, time.Now())

	var orderData ordermodel.MerchantPayOutOrderM
	orderData, err = s.orderDao.GetByOrderId(orderIndexTable, indexTableResult.OrderID)
	if err != nil {
		return resp, err
	}

	resp.Status = utils.ConvertOrderStatus(orderData.Status)
	resp.TranFlow = orderData.MOrderID
	resp.PaySerialNo = strconv.FormatUint(orderData.OrderID, 10)
	resp.Amount = orderData.Amount.String()
	resp.Code = string('0')

	return resp, nil
}

// SelectPaymentChannel 根据商户和订单金额选择可用通道
func (s *PayoutOrderService) SelectPaymentChannel(merchantID uint, amount decimal.Decimal, channelCode string, currency string) (*dto.PaymentChannelVo, error) {

	var payRouteList []dto.PaymentChannelVo
	// 查询商户路由
	mainDao := &dao.MainDao{}
	payRouteList, _ = mainDao.SelectPaymentChannel(merchantID, channelCode, currency)
	if len(payRouteList) < 1 {
		return nil, errors.New("没有可用通道")
	}

	for _, route := range payRouteList {
		var channel dto.PaymentChannelVo

		channel = route
		// 0. 检查金额是否符合通道 orderRange
		if !utils.MatchOrderRange(amount, channel.OrderRange) {
			continue
		}

		// 满足条件，返回该通道
		return &channel, nil
	}

	return nil, fmt.Errorf("no available payment channel")
}

// SelectPaymentChannel 查询系统通道编码
func (s *PayoutOrderService) QuerySysChannel(channelCode string) (dto.PayWayVo, error) {

	var payWayDetail dto.PayWayVo
	// 查询商户路由
	mainDao := &dao.MainDao{}
	payWayDetail, err := mainDao.GetSysChannel(channelCode)
	if err != nil {
		return payWayDetail, errors.New("通道编码不存在")
	}

	return payWayDetail, nil
}

func (s *PayoutOrderService) GetMerchantInfo(appId string) (uint64, error) {

	var merchant *mainmodel.Merchant
	// 1) 主库校验
	merchant, err := s.mainDao.GetMerchant(appId)
	if err != nil || merchant == nil || merchant.Status != 1 {
		return 0, errors.New("merchant invalid")
	}

	return merchant.MerchantID, nil
}
