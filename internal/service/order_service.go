package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"
	"wht-order-api/internal/utils"

	"wht-order-api/internal/dal"
	"wht-order-api/internal/dao"
	"wht-order-api/internal/dto"
	"wht-order-api/internal/idgen"
	ordermodel "wht-order-api/internal/model/order"
)

type OrderService struct {
	mainDao  *dao.MainDao
	orderDao *dao.OrderDao
}

func NewOrderService() *OrderService {
	return &OrderService{
		mainDao:  &dao.MainDao{},
		orderDao: &dao.OrderDao{},
	}
}

// 分片表名生成器：p_order_{YYYYMM}_p{orderID % 4}
func getOrderTable(base string, orderID uint64, t time.Time) string {
	month := t.Format("200601")
	shard := orderID % 4
	return fmt.Sprintf("%s_%s_p%d", base, month, shard)
}

func (s *OrderService) Create(req dto.CreateOrderReq) (dto.CreateOrderResp, error) {
	var response dto.CreateOrderResp
	// 1) 主库校验
	merchant, err := s.mainDao.GetMerchant(req.MerchantNo)
	if err != nil || merchant == nil || merchant.Status != 1 {
		return response, errors.New("merchant invalid")
	}
	amount, err := strconv.ParseFloat(req.Amount, 64)
	if err != nil {
		return response, errors.New("金额格式错误")
	}
	// 查询系统通道是否有效
	channelDtail, err := s.QuerySysChannel(req.PayType)
	if err != nil {
		return response, errors.New("通道不存在")
	}
	log.Printf("请求通道编码: %v", req.PayType)
	// 1) 选择可用上游通道
	channel, err := s.SelectPaymentChannel(uint(merchant.MerchantID), amount, req.PayType, channelDtail.Currency)
	if err != nil || channel.Coding == "" {
		return response, errors.New("没有可用通道")
	}

	now := time.Now()
	// 2) 全局订单ID
	oid := idgen.New()
	table := getOrderTable("p_order", oid, now)

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
	//var agentPct, agentFixed float64 = 0, 0
	//if merchant.PId > 0 {
	//	var agentMerchant dto.QueryAgentMerchant
	//	agentMerchant.AId = int64(merchant.PId)
	//	agentMerchant.MId = int64(merchant.MerchantID)
	//	agentMerchant.SysChannelID = channel.SysChannelID
	//	agentMerchant.UpChannelID = channel.UpChannelID
	//	agentMerchant.Currency = channel.Currency
	//	agentInfo, err := s.mainDao.GetAgentMerchant(agentMerchant)
	//	if err != nil && agentInfo != nil && agentInfo.Status == 1 {
	//		agentPct = agentInfo.DefaultRate
	//		agentFixed = agentInfo.SingleFee
	//	}
	//}
	// 订单金额转化浮点数
	orderAmount, err := strconv.ParseFloat(req.Amount, 64)
	if err != nil {
		return response, errors.New("无效的浮点数")
	}
	//settle := utils.Calculate(orderAmount, channel.MDefaultRate, channel.MSingleFee, agentPct, agentFixed, channel.UpDefaultRate, channel.UpSingleFee, "agent_from_platform")

	// 6) 构造订单模型
	m := &ordermodel.MerchantOrder{
		OrderID:     oid,
		MID:         merchant.MerchantID,
		MOrderID:    req.TranFlow,
		Amount:      orderAmount,
		Currency:    req.Currency,
		SupplierID:  channel.UpstreamId,
		Status:      0,
		NotifyURL:   req.NotifyUrl,
		ReturnURL:   req.RedirectUrl,
		ChannelID:   channel.SysChannelID,
		ChannelCode: &channel.Coding,
		Title:       req.ProductInfo,
		PayEmail:    req.PayEmail,
		PayPhone:    req.PayPhone,
	}

	// 7) 插入数据库
	if err := s.orderDao.Insert(table, m); err != nil {
		return response, err
	}

	txId := idgen.New()
	txTable := getOrderTable("p_up_order", txId, time.Now())
	// 5. 写入上游流水
	tx := &ordermodel.UpstreamTx{
		OrderID:    oid,
		MerchantID: merchant.MerchantID,
		SupplierId: uint64(channel.UpstreamId),
		Amount:     req.Amount,
		Currency:   req.Currency,
		Status:     0,
		UpOrderId:  txId,
	}

	// 7) 插入数据库
	if err := s.orderDao.InsertTx(txTable, tx); err != nil {
		return response, err
	}

	// 4) 调用上游下单接口生成支付链接

	var upstreamRequest dto.UpstreamRequest
	upstreamRequest.Currency = req.Currency
	upstreamRequest.Amount = req.Amount
	upstreamRequest.RedirectUrl = req.RedirectUrl
	upstreamRequest.ProductInfo = req.ProductInfo
	upstreamRequest.PayType = req.PayType
	upstreamRequest.Currency = channel.Currency
	upstreamRequest.ProviderKey = channel.ChannelCode
	upstreamRequest.MchOrderId = strconv.FormatUint(txId, 10)
	upstreamRequest.ApiKey = merchant.ApiKey
	upstreamRequest.MchNo = channel.UpAccount
	upstreamRequest.NotifyUrl = req.NotifyUrl

	mOrderId, upOrderNo, payUrl, err := CallUpstreamService(upstreamRequest, channel)
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

	response.Yul1 = payUrl
	response.PaySerialNo = string(oid) //订单表编号
	response.TranFlow = req.TranFlow
	response.SysTime = time.Now().String()
	response.Amount = req.Amount

	// 9) 缓存到 Redis
	cacheKey := "order:" + strconv.FormatUint(oid, 10)
	_ = dal.RedisClient.Set(dal.RedisCtx, cacheKey, utils.MapToJSON(m), 10*time.Minute).Err()

	// 10) 发布 MQ 事件
	//evt := mq.OrderCreatedEvent{
	//	OrderID: oid, MerchantID: merchant.MerchantID, MerchantOrdNo: req.TranFlow,
	//	Amount: req.Amount, Currency: req.Currency, PayMethod: "", CreatedAt: now.Unix(),
	//}
	//_ = mq.PublishOrderCreated(evt)
	response.Code = string(0)
	return response, nil
}

func (s *OrderService) Get(id uint64) (*ordermodel.MerchantOrder, error) {
	// 优先读 Redis
	cacheKey := "order:" + strconv.FormatUint(id, 10)
	if sjson, err := dal.RedisClient.Get(dal.RedisCtx, cacheKey).Result(); err == nil {
		var mo ordermodel.MerchantOrder
		if err := json.Unmarshal([]byte(sjson), &mo); err == nil {
			return &mo, nil
		}
	}

	// fallback DB：按 ID 推导分片表
	table := getOrderTable("p_order", id, time.Now())
	return s.orderDao.GetByID(table, id)
}

// SelectPaymentChannel 根据商户和订单金额选择可用通道
func (s *OrderService) SelectPaymentChannel(merchantID uint, amount float64, channelCode string, currency string) (*dto.PaymentChannelVo, error) {

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
func (s *OrderService) QuerySysChannel(channelCode string) (dto.PayWayVo, error) {

	var payWayDetail dto.PayWayVo
	// 查询商户路由
	mainDao := &dao.MainDao{}
	payWayDetail, err := mainDao.GetSysChannel(channelCode)
	if err != nil {
		return payWayDetail, errors.New("通道编码不存在")
	}

	return payWayDetail, nil
}
