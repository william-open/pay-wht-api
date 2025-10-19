package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/jinzhu/copier"
	"gorm.io/gorm"
	"log"
	"runtime/debug"
	"sort"
	"strconv"
	"sync"
	"time"
	"wht-order-api/internal/notify"
	"wht-order-api/internal/shard"
	"wht-order-api/internal/system"

	"github.com/shopspring/decimal"
	"golang.org/x/sync/singleflight"

	"wht-order-api/internal/channel/health"
	mainmodel "wht-order-api/internal/model/main"
	"wht-order-api/internal/utils"

	"wht-order-api/internal/dal"
	"wht-order-api/internal/dao"
	"wht-order-api/internal/dto"
	"wht-order-api/internal/idgen"
	ordermodel "wht-order-api/internal/model/order"
)

// ================== Redis 失败计数 ==================
const upstreamFailKey = "pay_up_fail:"

type ReceiveOrderService struct {
	mainDao       *dao.MainDao  // 主数据库
	orderDao      *dao.OrderDao //订单数据库
	indexTableDao *dao.IndexTableDao
	merchantGroup singleflight.Group
	channelGroup  singleflight.Group
	ctx           context.Context
	cancel        context.CancelFunc
}

func NewReceiveOrderService() *ReceiveOrderService {
	ctx, cancel := context.WithCancel(context.Background())
	return &ReceiveOrderService{
		mainDao:       dao.NewMainDao(),
		orderDao:      dao.NewOrderDao(), // 默认全局 DB
		indexTableDao: dao.NewIndexTableDao(),
		ctx:           ctx,
		cancel:        cancel,
	}
}

func (s *ReceiveOrderService) Shutdown() {
	s.cancel()
}

// 记录失败
func (s *ReceiveOrderService) recordUpstreamFail(upstreamID uint64) {
	key := fmt.Sprintf("%s%d", upstreamFailKey, upstreamID)
	cnt, _ := dal.RedisClient.Incr(dal.RedisCtx, key).Result()
	if cnt == 1 {
		dal.RedisClient.Expire(dal.RedisCtx, key, 5*time.Minute)
	}
	if cnt == 3 {
		notify.Notify(system.BotChatID, "warn", "通道降权提醒",
			fmt.Sprintf("⚠️ 上游通道 %d 在5分钟内失败 ≥3次，权重减半", upstreamID), false)
	}
	if cnt >= 10 {
		notify.Notify(system.BotChatID, "error", "上游通道告警",
			fmt.Sprintf("🚨 上游通道 %d 在5分钟内失败次数已达 %d 次", upstreamID, cnt), true)
	}
}

// 清理失败计数
func (s *ReceiveOrderService) clearUpstreamFail(upstreamID uint64) {
	key := fmt.Sprintf("%s%d", upstreamFailKey, upstreamID)
	dal.RedisClient.Del(dal.RedisCtx, key)
}

// 获取失败次数
func (s *ReceiveOrderService) getUpstreamFailCount(upstreamID uint64) int {
	key := fmt.Sprintf("%s%d", upstreamFailKey, upstreamID)
	val, _ := dal.RedisClient.Get(dal.RedisCtx, key).Result()
	if val == "" {
		return 0
	}
	cnt, _ := strconv.Atoi(val)
	return cnt
}

// Create 处理代收订单下单业务逻辑（高并发优化版）
// ================== Create 主流程 ==================
func (s *ReceiveOrderService) Create(req dto.CreateOrderReq) (resp dto.CreateOrderResp, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[PANIC] Create order panic: %v\n%s", r, debug.Stack())
			notify.Notify(system.BotChatID, "error", "系统Panic", fmt.Sprintf("panic: %v", r), true)
			resp = dto.CreateOrderResp{
				TranFlow: req.TranFlow, Amount: req.Amount,
				Code: "999", Status: "9999", SysTime: strconv.FormatInt(utils.GetTimestampMs(), 10),
			}
			err = fmt.Errorf("internal error")
		}
	}()

	// 1 参数验证
	if err = validateCreateRequest(req); err != nil {
		return resp, err
	}

	// 2 商户信息
	merchant, err := s.getMerchantWithCache(req.MerchantNo)
	if err != nil || merchant == nil {
		return resp, fmt.Errorf("merchant invalid: %w", err)
	}

	// 3 金额
	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		return resp, errors.New("amount format error")
	}

	// 4 通道信息
	channelDetail, err := s.getSysChannelWithCache(req.PayType)
	if err != nil || channelDetail == nil {
		return resp, errors.New("channel invalid")
	}

	// 5 商户通道
	merchantChannelInfo, err := NewCommonService().GetMerchantChannelInfo(merchant.MerchantID, req.PayType)
	if err != nil || merchantChannelInfo == nil {
		return resp, errors.New("merchant channel invalid")
	}

	// 6 选择通道
	var products []dto.PayProductVo
	if req.PayProductId != "" {
		// 先转成 uint64，再强转成 uint
		payProductId, err := strconv.ParseUint(req.PayProductId, 10, 64)
		if err != nil {
			fmt.Println("转换失败:", err)
			return resp, errors.New("test admin no single channel available,pay_product_id transfer error")
		}
		single, err := s.TestSelectSingleChannel(uint(merchant.MerchantID), req.PayType, 1, channelDetail.Currency, payProductId)
		if err != nil {
			return resp, errors.New("admin test no single channel available")
		}
		// 检查金额是否在通道允许范围内
		orderRange := fmt.Sprintf("%v-%v", single.MinAmount, single.MaxAmount)
		if !utils.MatchOrderRange(amount, orderRange) {
			return resp, errors.New(fmt.Sprintf("admin test the amount does not meet the risk control requirements.order amount:%v,limit amount:%s", amount, orderRange)) // 金额不符合风控要求，跳过
		}
		products = []dto.PayProductVo{single}
	} else {
		if merchantChannelInfo.DispatchMode == 2 {
			single, err := s.SelectSingleChannel(uint(merchant.MerchantID), req.PayType, 1, channelDetail.Currency)
			if err != nil {
				return resp, errors.New("no single channel available")
			}
			// 检查金额是否在通道允许范围内
			orderRange := fmt.Sprintf("%v-%v", single.MinAmount, single.MaxAmount)
			if !utils.MatchOrderRange(amount, orderRange) {
				return resp, errors.New(fmt.Sprintf("the amount does not meet the risk control requirements..order amount:%v,limit amount:%s", amount, orderRange)) // 金额不符合风控要求，跳过
			}
			products = []dto.PayProductVo{single}
		} else {
			products, err = s.selectPollingChannelWithRetry(uint(merchant.MerchantID), req.PayType, 1, channelDetail.Currency, amount)
			if err != nil {
				return resp, err
			}
		}
	}

	// 7 幂等检查
	oid, exists, err := s.checkIdempotency(merchant.MerchantID, req.TranFlow)
	if err != nil {
		return resp, err
	}
	if exists {
		return resp, nil
	}

	// 8 计算结算
	settle, err := s.calculateSettlement(merchant, products[0], amount)
	if err != nil {
		return resp, err
	}

	// 9 创建订单
	now := time.Now()
	order, tx, err := s.createOrderAndTransaction(merchant, req, products[0], amount, oid, now, settle)
	if err != nil {
		return resp, err
	}

	// 10 调用上游（失败降级）
	var payUrl string
	var lastErr error
	for _, product := range products {
		payUrl, err = s.callUpstreamService(merchant, &req, &product, tx.UpOrderId)
		if err == nil {
			s.clearUpstreamFail(uint64(product.UpstreamId))
			// 更新成功率（异步）
			go func(pid int64) {
				if e := s.mainDao.UpdateSuccessRate(pid, true); e != nil {
					log.Printf("update channel success rate failed: %v", e)
				}
			}(product.ID)
			break
		}

		// 更新通道成功率（异步）
		go func(pid int64) {
			if e := s.mainDao.UpdateSuccessRate(pid, false); e != nil {
				log.Printf("update channel success rate failed: %v", e)
			}
		}(product.ID)

		// 记录失败计数
		s.recordUpstreamFail(uint64(product.UpstreamId))

		// ⚠️ 每次失败后都发 Telegram
		notify.Notify(system.BotChatID, "warn", "代收上游调用失败",
			fmt.Sprintf("\n商户号: %s\n通道编码: %s\n上游通道: %s\n上游接口: %s\n供应商: %s\n订单号: %s\n失败原因: %v\n商户请求参数: %s",
				req.MerchantNo,
				req.PayType,
				product.UpChannelTitle,
				product.InterfaceCode,
				product.UpstreamTitle,
				req.TranFlow,
				err,
				utils.MapToJSON(req),
			), true)

		lastErr = err
	}

	if payUrl == "" && lastErr != nil {
		resp = dto.CreateOrderResp{
			TranFlow: req.TranFlow, PaySerialNo: strconv.FormatUint(oid, 10),
			Amount: req.Amount, Code: "001", SysTime: strconv.FormatInt(utils.GetTimestampMs(), 10),
		}
		return resp, lastErr
	}

	// 11 构建响应
	resp = dto.CreateOrderResp{
		TranFlow: req.TranFlow, PaySerialNo: strconv.FormatUint(oid, 10),
		Amount: req.Amount, Code: "0", Status: "0001",
		SysTime: strconv.FormatInt(utils.GetTimestampMs(), 10), Yul1: payUrl,
	}

	// 12 异步事件
	go s.asyncPostOrderCreation(oid, order, merchant.MerchantID, req.TranFlow, req.Amount, now)
	return resp, nil
}

// validateCreateRequest 验证创建订单请求
func validateCreateRequest(req dto.CreateOrderReq) error {
	if req.MerchantNo == "" {
		return errors.New("merchantNo is required")
	}
	if req.TranFlow == "" {
		return errors.New("tranFlow is required")
	}
	if req.Amount == "" {
		return errors.New("amount is required")
	}
	if req.PayType == "" {
		return errors.New("payType is required")
	}
	return nil
}

// getMerchantWithCache 获取商户信息（带缓存和防击穿）
func (s *ReceiveOrderService) getMerchantWithCache(merchantNo string) (*mainmodel.Merchant, error) {
	key := "merchant:" + merchantNo

	// 使用singleflight防止缓存击穿
	result, err, _ := s.merchantGroup.Do(key, func() (interface{}, error) {
		// 尝试从缓存获取
		cacheKey := "merchant_cache:" + merchantNo
		cached, err := dal.RedisClient.Get(dal.RedisCtx, cacheKey).Result()
		if err == nil && cached != "" {
			var merchant mainmodel.Merchant
			if err := utils.JSONToMap(cached, &merchant); err == nil {
				return &merchant, nil
			}
		}

		// 从数据库获取
		merchant, err := s.mainDao.GetMerchant(merchantNo)
		if err != nil || merchant == nil || merchant.Status != 1 {
			return nil, errors.New(fmt.Sprintf("[%v]merchant not found or invalid", merchantNo))
		}

		// 缓存结果
		merchantJSON := utils.MapToJSON(merchant)
		dal.RedisClient.Set(dal.RedisCtx, cacheKey, merchantJSON, 5*time.Minute)

		return merchant, nil
	})

	if err != nil {
		return nil, err
	}

	return result.(*mainmodel.Merchant), nil
}

// getSysChannelWithCache 获取系统通道信息（带缓存）
func (s *ReceiveOrderService) getSysChannelWithCache(channelCode string) (*dto.PayWayVo, error) {
	key := "sys_channel:" + channelCode

	result, err, _ := s.channelGroup.Do(key, func() (interface{}, error) {
		// 尝试从缓存获取
		cacheKey := "sys_channel_cache:" + channelCode
		cached, err := dal.RedisClient.Get(dal.RedisCtx, cacheKey).Result()
		if err == nil && cached != "" {
			var channel *dto.PayWayVo
			if err := utils.JSONToMap(cached, &channel); err == nil {
				return channel, nil
			}
		}

		// 从数据库获取
		channel, err := s.mainDao.GetSysChannel(channelCode)
		if err != nil {
			return &dto.PayWayVo{}, errors.New("channel not found")
		}

		// 缓存结果
		channelJSON := utils.MapToJSON(channel)
		dal.RedisClient.Set(dal.RedisCtx, cacheKey, channelJSON, 10*time.Minute)

		return channel, nil
	})

	if err != nil {
		return &dto.PayWayVo{}, err
	}

	return result.(*dto.PayWayVo), nil
}

// ================== 轮询通道选择（权重优先 + 失败降级） ==================
func (s *ReceiveOrderService) selectPollingChannelWithRetry(
	merchantID uint, sysChannelCode string, channelType int8, currency string, amount decimal.Decimal,
) ([]dto.PayProductVo, error) {
	products, err := s.mainDao.GetAvailablePollingPayProducts(merchantID, sysChannelCode, currency, channelType)
	if err != nil || len(products) == 0 {
		return nil, errors.New("no channel products available")
	}
	for i := range products {
		failCnt := s.getUpstreamFailCount(uint64(products[i].UpstreamId))
		if failCnt >= 3 {
			products[i].UpstreamWeight = max(1, products[i].UpstreamWeight/2)
		}
	}
	sort.SliceStable(products, func(i, j int) bool {
		return products[i].UpstreamWeight > products[j].UpstreamWeight
	})
	return products, nil
}

// selectPollingChannelWithRetry 带重试的轮询通道选择
//func (s *ReceiveOrderService) selectPollingChannelWithRetry(mId uint, sysChannelCode string, channelType int8, currency string, amount decimal.Decimal) (dto.PayProductVo, error) {
//	// 获取健康管理器
//	healthManager := s.getHealthManager()
//
//	// 获取可用通道产品
//	products, err := s.mainDao.GetAvailablePollingPayProducts(mId, sysChannelCode, currency, channelType)
//	if err != nil || len(products) == 0 {
//		return dto.PayProductVo{}, errors.New("no channel products available")
//	}
//
//	// 按权重降序排序
//	sort.SliceStable(products, func(i, j int) bool {
//		return products[i].UpstreamWeight > products[j].UpstreamWeight
//	})
//
//	// 尝试找到合适的通道
//	for _, product := range products {
//		// 跳过禁用的通道
//		if healthManager.IsDisabled(product.ID) {
//			continue
//		}
//
//		// 检查费率
//		if product.MDefaultRate.LessThanOrEqual(product.CostRate) {
//			continue
//		}
//
//		// 检查金额范围
//		orderRange := fmt.Sprintf("%v-%v", product.MinAmount, product.MaxAmount)
//		if !utils.MatchOrderRange(amount, orderRange) {
//			continue
//		}
//
//		return product, nil
//	}
//	return dto.PayProductVo{}, errors.New("polling channel,no suitable channel found after filtering")
//}

// getHealthManager 获取通道健康管理器
func (s *ReceiveOrderService) getHealthManager() *health.ChannelHealthManager {
	return &health.ChannelHealthManager{
		Redis:     dal.RedisClient,
		Strategy:  &health.DecayStrategy{Factor: 0.95},
		Threshold: 60.0,
		TTL:       30 * time.Minute,
	}
}

// checkIdempotency 检查幂等性
func (s *ReceiveOrderService) checkIdempotency(merchantID uint64, tranFlow string) (uint64, bool, error) {
	oid := idgen.New()
	table := shard.OrderShard.GetTable(oid, time.Now())

	// 检查是否已存在订单
	exist, err := s.orderDao.GetByMerchantNo(table, merchantID, tranFlow)
	if err != nil {
		return 0, false, err
	}
	if exist != nil {
		return 0, true, nil
	}

	return oid, false, nil
}

// calculateSettlement 计算结算费用
func (s *ReceiveOrderService) calculateSettlement(merchant *mainmodel.Merchant, payChannelProduct dto.PayProductVo, amount decimal.Decimal) (dto.SettlementResult, error) {
	var agentPct, agentFixed = decimal.Zero, decimal.Zero

	// 如果有代理商户，获取代理信息
	if merchant.PId > 0 {
		agentMerchant := dto.QueryAgentMerchant{
			AId:          int64(merchant.PId),
			MId:          int64(merchant.MerchantID),
			SysChannelID: payChannelProduct.SysChannelID,
			Currency:     payChannelProduct.Currency,
		}

		agentInfo, err := s.mainDao.GetAgentMerchant(agentMerchant)
		if err == nil && agentInfo != nil && agentInfo.Status == 1 {
			agentPct = agentInfo.DefaultRate
			agentFixed = agentInfo.SingleFee
		}
	}

	// 计算结算费用
	settle := utils.Calculate(
		amount,
		payChannelProduct.MDefaultRate,
		payChannelProduct.MSingleFee,
		agentPct,
		agentFixed,
		payChannelProduct.CostRate,
		payChannelProduct.CostFee,
		"agent_from_platform",
		payChannelProduct.Currency,
	)

	return settle, nil
}

// createOrderAndTransaction 创建订单和事务
func (s *ReceiveOrderService) createOrderAndTransaction(
	merchant *mainmodel.Merchant,
	req dto.CreateOrderReq,
	payChannelProduct dto.PayProductVo,
	amount decimal.Decimal,
	oid uint64,
	now time.Time,
	settle dto.SettlementResult,
) (*ordermodel.MerchantOrder, *ordermodel.UpstreamTx, error) {
	var order *ordermodel.MerchantOrder
	var tx *ordermodel.UpstreamTx

	err := dal.OrderDB.Transaction(func(txDB *gorm.DB) error {
		// 事务内的 dao
		orderDao := dao.NewOrderDaoWithDB(txDB)

		// 创建订单
		if err := s.createOrder(merchant, req, payChannelProduct, amount, oid, now, settle, orderDao); err != nil {
			return err
		}

		// 创建上游事务
		upTx, err := s.createUpstreamTx(merchant, req, payChannelProduct, oid, now, orderDao)
		if err != nil {
			return err
		}
		tx = upTx

		// 创建索引
		if err := s.createOrderIndex(merchant, req, oid, now, orderDao); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	// 查询订单和上游事务
	orderTable := shard.OrderShard.GetTable(oid, now)
	order, err = s.orderDao.GetByOrderId(orderTable, oid)
	if err != nil {
		return nil, nil, err
	}

	txTable := shard.UpOrderShard.GetTable(tx.UpOrderId, now)
	tx, err = s.orderDao.GetTxByOrderId(txTable, oid)
	if err != nil {
		return nil, nil, err
	}

	return order, tx, nil
}

// createOrder 创建订单
func (s *ReceiveOrderService) createOrder(
	merchant *mainmodel.Merchant,
	req dto.CreateOrderReq,
	payChannelProduct dto.PayProductVo,
	amount decimal.Decimal,
	oid uint64,
	now time.Time,
	settle dto.SettlementResult,
	orderDao *dao.OrderDao, // 使用事务 Dao
) error {
	var orderSettle dto.SettlementResult
	if err := copier.Copy(&orderSettle, &settle); err != nil {
		return err
	}

	log.Printf(">>>支付产品信息:%+v", payChannelProduct)
	costFee := amount.Mul(payChannelProduct.CostRate).Div(decimal.NewFromInt(100))      //上游成本费用
	orderFee := amount.Mul(payChannelProduct.MDefaultRate).Div(decimal.NewFromInt(100)) //商户手续费
	profitFee := orderFee.Sub(costFee)
	m := &ordermodel.MerchantOrder{
		OrderID:        oid,
		MID:            merchant.MerchantID,
		MOrderID:       req.TranFlow,
		Amount:         amount,
		Currency:       payChannelProduct.Currency,
		SupplierID:     payChannelProduct.UpstreamId,
		Status:         1,
		NotifyURL:      req.NotifyUrl,
		ReturnURL:      req.RedirectUrl,
		ChannelID:      payChannelProduct.SysChannelID,
		UpChannelID:    payChannelProduct.ID,
		ChannelCode:    &payChannelProduct.SysChannelCode,
		Title:          req.ProductInfo,
		PayEmail:       req.PayEmail,
		PayPhone:       req.PayPhone,
		MTitle:         &merchant.NickName,
		ChannelTitle:   &payChannelProduct.SysChannelTitle,
		UpChannelCode:  &payChannelProduct.UpstreamCode,
		UpChannelTitle: &payChannelProduct.UpChannelTitle,
		MRate:          &payChannelProduct.MDefaultRate,
		UpRate:         &payChannelProduct.CostRate,
		MFixedFee:      &payChannelProduct.MSingleFee,
		UpFixedFee:     &payChannelProduct.CostFee,
		Fees:           settle.MerchantTotalFee,
		Country:        &payChannelProduct.Country,
		Cost:           &costFee,
		Profit:         &profitFee,
		SettleSnapshot: ordermodel.SettleSnapshot(orderSettle),
		CreateTime:     &now,
		AID: func() uint64 {
			if merchant.PId > 0 {
				return merchant.PId
			}
			return 0
		}(),
	}

	table := shard.OrderShard.GetTable(oid, now)
	return orderDao.Insert(table, m)
}

// createUpstreamTx 创建上游事务
func (s *ReceiveOrderService) createUpstreamTx(
	merchant *mainmodel.Merchant,
	req dto.CreateOrderReq,
	payChannelProduct dto.PayProductVo,
	oid uint64,
	now time.Time,
	orderDao *dao.OrderDao,
) (*ordermodel.UpstreamTx, error) {
	txId := idgen.New()
	txTable := shard.UpOrderShard.GetTable(txId, now)

	tx := &ordermodel.UpstreamTx{
		OrderID:    oid,
		MerchantID: strconv.FormatUint(merchant.MerchantID, 10),
		SupplierId: uint64(payChannelProduct.UpstreamId),
		Amount:     utils.MustStringToDecimal(req.Amount),
		Currency:   payChannelProduct.Currency,
		Status:     0,
		UpOrderId:  txId,
		CreateTime: &now,
		UpdateTime: &now,
	}

	if err := orderDao.InsertTx(txTable, tx); err != nil {
		return nil, err
	}

	// 更新订单表
	updateOrder := dto.UpdateOrderVo{
		OrderId:    oid,
		UpOrderId:  txId,
		UpdateTime: now,
	}

	orderTable := shard.OrderShard.GetTable(oid, now)
	if err := orderDao.UpdateOrder(orderTable, updateOrder); err != nil {
		return nil, err
	}

	return tx, nil
}

// createOrderIndex 创建订单索引
func (s *ReceiveOrderService) createOrderIndex(
	merchant *mainmodel.Merchant,
	req dto.CreateOrderReq,
	oid uint64,
	now time.Time,
	orderDao *dao.OrderDao,
) error {
	receiveIndexTable := utils.GetOrderIndexTable("p_order_index", now)
	orderLogIndexTable := shard.OrderLogShard.GetTable(oid, now)
	receiveLogIndexTable := shard.OrderShard.GetTable(oid, now)

	receiveIndex := &ordermodel.ReceiveOrderIndexM{
		MID:               merchant.MerchantID,
		MOrderID:          req.TranFlow,
		OrderID:           oid,
		OrderTableName:    receiveLogIndexTable,
		OrderLogTableName: orderLogIndexTable,
		CreateTime:        now,
	}

	return orderDao.InsertReceiveOrderIndexTable(receiveIndexTable, receiveIndex)
}

// callUpstreamService 调用上游服务
func (s *ReceiveOrderService) callUpstreamService(
	merchant *mainmodel.Merchant,
	req *dto.CreateOrderReq,
	payChannelProduct *dto.PayProductVo,
	txId uint64,
) (string, error) {
	if payChannelProduct == nil {
		return "", errors.New(" callUpstreamService pay product cannot be nil")
	}
	if merchant == nil {
		return "", errors.New("callUpstreamService merchant cannot be nil")
	}
	if req == nil {
		return "", errors.New("callUpstreamService req cannot be nil")
	}
	log.Printf("[Upstream-Receive-origin],请求参数: %+v", req)
	upstreamRequest := dto.UpstreamRequest{
		Currency:     payChannelProduct.Currency,
		Amount:       req.Amount,
		RedirectUrl:  req.RedirectUrl,
		ProductInfo:  req.ProductInfo,
		PayType:      req.PayType,
		AccNo:        req.AccNo,
		AccName:      req.AccName,
		PayPhone:     req.PayPhone,
		ProviderKey:  payChannelProduct.InterfaceCode,
		MchOrderId:   strconv.FormatUint(txId, 10),
		ApiKey:       payChannelProduct.UpApiKey,
		SubmitUrl:    payChannelProduct.PayApi,
		QueryUrl:     payChannelProduct.PayoutQueryApi,
		MchNo:        payChannelProduct.UpAccount,
		UpstreamCode: payChannelProduct.UpstreamCode,
		IdentityType: req.IdentityType,
		IdentityNum:  req.IdentityNum,
		BankCode:     req.BankCode,
		BankName:     req.BankName,
		PayMethod:    req.PayMethod,
		PayEmail:     req.PayEmail,
		NotifyUrl:    req.NotifyUrl,
		Mode:         "receive",
		ClientIp:     req.ClientId,
	}

	// 使用带超时的上下文
	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
	defer cancel()

	// 调用上游服务
	mOrderId, upOrderNo, payUrl, err := CallUpstreamReceiveService(ctx, upstreamRequest)
	if err != nil {
		return "", err
	}

	// 更新上游交易订单信息
	if mOrderId != "" {
		mOrderIdUint, err := strconv.ParseUint(mOrderId, 10, 64)
		if err != nil {
			log.Printf("上游订单号转换失败: %v", err)
		} else {
			txTable := shard.UpOrderShard.GetTable(txId, time.Now())
			upTx := dto.UpdateUpTxVo{
				UpOrderId: mOrderIdUint,
				UpOrderNo: upOrderNo,
			}

			if err := s.orderDao.UpdateUpTx(txTable, upTx); err != nil {
				log.Printf("更新上游交易失败: %v", err)
			}
		}
	}

	return payUrl, nil
}

// asyncPostOrderCreation 异步处理订单创建后的操作
func (s *ReceiveOrderService) asyncPostOrderCreation(oid uint64, order *ordermodel.MerchantOrder, merchantID uint64, tranFlow, amount string, now time.Time) {
	// 缓存到 Redis
	cacheKey := "order:" + strconv.FormatUint(oid, 10)
	if err := dal.RedisClient.Set(dal.RedisCtx, cacheKey, utils.MapToJSON(order), 10*time.Minute).Err(); err != nil {
		log.Printf("缓存订单失败: %v", err)
	}
}

// Get 代收订单查询
func (s *ReceiveOrderService) Get(param dto.QueryReceiveOrderReq) (dto.QueryReceiveOrderResp, error) {
	var resp dto.QueryReceiveOrderResp

	// 参数验证
	if param.MerchantNo == "" {
		return resp, errors.New("merchantNo is required")
	}
	if param.TranFlow == "" {
		return resp, errors.New("tranFlow is required")
	}

	// 获取商户ID
	mId, err := s.GetMerchantInfo(param.MerchantNo)
	if err != nil {
		return resp, err
	}

	// 查询索引表
	indexTable := utils.GetOrderIndexTable("p_order_index", time.Now())
	indexTableResult, err := s.indexTableDao.GetByIndexTable(indexTable, param.TranFlow, mId)
	if err != nil {
		return resp, errors.New("order not found")
	}

	// 查询订单表
	orderTable := shard.OrderShard.GetTable(indexTableResult.OrderID, time.Now())
	orderData, err := s.orderDao.GetByOrderId(orderTable, indexTableResult.OrderID)
	if err != nil {
		return resp, err
	}

	// 构建响应
	resp.Status = utils.ConvertOrderStatus(orderData.Status)
	resp.TranFlow = orderData.MOrderID
	resp.PaySerialNo = strconv.FormatUint(orderData.OrderID, 10)
	resp.Amount = orderData.Amount.String()
	resp.Code = "0"

	return resp, nil
}

// TestSelectSingleChannel 查询单独支付通道
func (s *ReceiveOrderService) TestSelectSingleChannel(mId uint, sysChannelCode string, channelType int8, currency string, payProductId uint64) (dto.PayProductVo, error) {
	// 查询单独支付通道产品
	payDetail, err := s.mainDao.GetTestSinglePayChannel(mId, sysChannelCode, channelType, currency, payProductId)

	if err != nil {
		return dto.PayProductVo{}, fmt.Errorf(" test admin get single pay channel failed: %w", err)
	}

	return payDetail, nil
}

// SelectSingleChannel 查询单独支付通道
func (s *ReceiveOrderService) SelectSingleChannel(mId uint, sysChannelCode string, channelType int8, currency string) (dto.PayProductVo, error) {

	// 查询单独支付通道产品
	payDetail, err := s.mainDao.GetSinglePayChannel(mId, sysChannelCode, channelType, currency)

	if err != nil {
		return payDetail, errors.New("no channel products available")
	}

	return payDetail, nil

}

// SelectPollingChannel 查询轮询所有支付通道
func (s *ReceiveOrderService) SelectPollingChannel(mId uint, sysChannelCode string, channelType int8, currency string, amount decimal.Decimal) ([]dto.PayProductVo, error) {
	// 查询所有可用通道产品（状态开启），按 weight 降序
	products, err := s.mainDao.GetAvailablePollingPayProducts(mId, sysChannelCode, currency, channelType)
	if err != nil || len(products) == 0 {
		return nil, errors.New("no channel products available")
	}

	// 按权重降序排序
	sort.SliceStable(products, func(i, j int) bool {
		return products[i].UpstreamWeight > products[j].UpstreamWeight
	})

	// 过滤符合风控金额和费率条件的通道
	var filtered []dto.PayProductVo
	for _, channel := range products {
		// 检查商户费率是否小于或等于通道成本费率
		if channel.MDefaultRate.LessThanOrEqual(channel.CostRate) {
			continue // 费率不合理，跳过
		}

		// 检查金额是否在通道允许范围内
		orderRange := fmt.Sprintf("%v-%v", channel.MinAmount, channel.MaxAmount)
		if !utils.MatchOrderRange(amount, orderRange) {
			continue // 金额不符合风控要求，跳过
		}

		// 满足条件，加入结果集
		filtered = append(filtered, channel)
	}

	if len(filtered) == 0 {
		return nil, errors.New("no suitable channel products after filtering")
	}

	return filtered, nil
}

// QuerySysChannel 查询系统通道编码
func (s *ReceiveOrderService) QuerySysChannel(channelCode string) (*dto.PayWayVo, error) {

	var payWayDetail *dto.PayWayVo
	// 查询商户路由
	mainDao := &dao.MainDao{}
	payWayDetail, err := mainDao.GetSysChannel(channelCode)
	if err != nil {
		return payWayDetail, errors.New("通道编码不存在")
	}

	return payWayDetail, nil
}

func (s *ReceiveOrderService) GetMerchantInfo(appId string) (uint64, error) {

	var merchant *mainmodel.Merchant
	// 1) 主库校验
	merchant, err := s.mainDao.GetMerchant(appId)
	if err != nil || merchant == nil || merchant.Status != 1 {
		return 0, errors.New("merchant invalid")
	}

	return merchant.MerchantID, nil
}

// SelectSingleChannel, SelectPollingChannel, QuerySysChannel, GetMerchantInfo 等方法

// BatchCreate 新增批量处理功能
func (s *ReceiveOrderService) BatchCreate(requests []dto.CreateOrderReq) ([]dto.CreateOrderResp, []error) {
	var wg sync.WaitGroup
	results := make([]dto.CreateOrderResp, len(requests))
	errors := make([]error, len(requests))

	// 使用工作池处理并发请求
	sem := make(chan struct{}, 50) // 限制并发数为50

	for i, req := range requests {
		wg.Add(1)
		sem <- struct{}{}

		go func(index int, request dto.CreateOrderReq) {
			defer wg.Done()
			defer func() { <-sem }()

			// 使用上下文超时控制
			ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
			defer cancel()

			// 创建带上下文的请求
			result, err := s.createWithContext(ctx, request)
			results[index] = result
			errors[index] = err
		}(i, req)
	}

	wg.Wait()
	return results, errors
}

// createWithContext 带上下文的创建方法
func (s *ReceiveOrderService) createWithContext(ctx context.Context, req dto.CreateOrderReq) (dto.CreateOrderResp, error) {
	// 使用select监听上下文超时或取消
	select {
	case <-ctx.Done():
		return dto.CreateOrderResp{}, ctx.Err()
	default:
		return s.Create(req)
	}
}
