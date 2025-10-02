package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/jinzhu/copier"
	"github.com/shopspring/decimal"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
	"log"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"wht-order-api/internal/channel/health"
	mainmodel "wht-order-api/internal/model/main"
	"wht-order-api/internal/notify"
	"wht-order-api/internal/shard"
	"wht-order-api/internal/system"
	"wht-order-api/internal/utils"

	"wht-order-api/internal/dal"
	"wht-order-api/internal/dao"
	"wht-order-api/internal/dto"
	"wht-order-api/internal/idgen"
	ordermodel "wht-order-api/internal/model/order"
)

// ================== Redis 失败计数 ==================
const payoutUpstreamFailKey = "payout_up_fail:"

type PayoutOrderService struct {
	mainDao         *dao.MainDao
	orderDao        *dao.PayoutOrderDao
	indexTableDao   *dao.IndexTableDao
	merchantGroup   singleflight.Group
	channelGroup    singleflight.Group
	upstreamGroup   singleflight.Group
	ctx             context.Context
	cancel          context.CancelFunc
	healthCheckLock sync.RWMutex
	lastHealthCheck time.Time
	isHealthy       bool
}

func NewPayoutOrderService() *PayoutOrderService {
	ctx, cancel := context.WithCancel(context.Background())
	service := &PayoutOrderService{
		mainDao:       dao.NewMainDao(),        // 使用工厂方法
		orderDao:      dao.NewPayoutOrderDao(), // 使用工厂方法
		indexTableDao: dao.NewIndexTableDao(),  // 使用工厂方法
		ctx:           ctx,
		cancel:        cancel,
		isHealthy:     false,
	}

	// 初始化时进行健康检查
	if service.HealthCheck() {
		service.isHealthy = true
		log.Println("PayoutOrderService 初始化成功")
	} else {
		log.Println("PayoutOrderService 初始化警告：健康检查未通过")
	}

	return service
}

func (s *PayoutOrderService) Shutdown() {
	s.cancel()
}

// ================== 上游失败监控 ==================
func (s *PayoutOrderService) recordUpstreamFail(upstreamID uint64) {
	key := fmt.Sprintf("%s%d", payoutUpstreamFailKey, upstreamID)
	cnt, _ := dal.RedisClient.Incr(dal.RedisCtx, key).Result()
	if cnt == 1 {
		dal.RedisClient.Expire(dal.RedisCtx, key, 5*time.Minute)
	}
	if cnt == 3 {
		notify.Notify(system.BotChatID, "warn", "代付通道降权提醒",
			fmt.Sprintf("⚠️ 代付通道 %d 在5分钟内失败 ≥3次，权重减半", upstreamID), false)
	}
	if cnt >= 10 {
		notify.Notify(system.BotChatID, "error", "代付通道告警",
			fmt.Sprintf("🚨 代付通道 %d 在5分钟内失败次数已达 %d 次", upstreamID, cnt), true)
	}
}

func (s *PayoutOrderService) clearUpstreamFail(upstreamID uint64) {
	key := fmt.Sprintf("%s%d", payoutUpstreamFailKey, upstreamID)
	dal.RedisClient.Del(dal.RedisCtx, key)
}

func (s *PayoutOrderService) getUpstreamFailCount(upstreamID uint64) int {
	key := fmt.Sprintf("%s%d", payoutUpstreamFailKey, upstreamID)
	val, _ := dal.RedisClient.Get(dal.RedisCtx, key).Result()
	if val == "" {
		return 0
	}
	cnt, _ := strconv.Atoi(val)
	return cnt
}

// ================== 轮询通道选择 ==================
func (s *PayoutOrderService) selectPollingChannels(
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

// Create 处理代付订单下单业务逻辑（增加了防 panic、判空、日志与堆栈打印）
func (s *PayoutOrderService) Create(req dto.CreatePayoutOrderReq) (resp dto.CreatePayoutOrderResp, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[PANIC] Payout Create panic: %v\n%s", r, debug.Stack())
			notify.Notify(system.BotChatID, "error", "代付Panic", fmt.Sprintf("panic: %v", r), true)
			resp = dto.CreatePayoutOrderResp{
				PaySerialNo: "", TranFlow: req.TranFlow, SysTime: time.Now().Format(time.RFC3339),
				Amount: req.Amount, Code: "999",
			}
			err = fmt.Errorf("internal error")
		}
	}()

	if !s.IsHealthy() {
		return resp, errors.New("service temporarily unavailable")
	}

	// 1 参数验证
	if err := validateCreatePayoutRequest(req); err != nil {
		return resp, err
	}

	// 2 商户
	merchant, err := s.getMerchantWithCache(req.MerchantNo)
	if err != nil || merchant == nil {
		return resp, fmt.Errorf("merchant invalid: %w", err)
	}

	// 3 金额
	amount, err := decimal.NewFromString(strings.TrimSpace(req.Amount))
	if err != nil || amount.LessThanOrEqual(decimal.Zero) {
		return resp, errors.New("amount invalid")
	}

	// 4 商户余额
	merchantMoney, mmErr := s.mainDao.GetMerchantAccount(strconv.FormatUint(merchant.MerchantID, 10))
	if mmErr != nil || merchantMoney.Money.LessThan(amount) {
		return resp, errors.New("insufficient balance")
	}

	// 5 系统通道
	channelDetail, err := s.getSysChannelWithCache(req.PayType)
	if err != nil || channelDetail == nil {
		return resp, errors.New("channel invalid")
	}

	// 6 商户通道
	merchantChannelInfo, err := NewCommonService().GetMerchantChannelInfo(merchant.MerchantID, req.PayType)
	if err != nil || merchantChannelInfo == nil {
		return resp, errors.New("merchant channel invalid")
	}

	// 7 选择通道
	var products []dto.PayProductVo
	if req.PayProductId != "" { // 管理后台测试用
		// 先转成 uint64，再强转成 uint
		payProductId, err := strconv.ParseUint(req.PayProductId, 10, 64)
		if err != nil {
			fmt.Println("转换失败:", err)
			return resp, errors.New("admin test no single channel available,pay_product_id transfer error")
		}
		single, err := s.TestSelectSingleChannel(uint(merchant.MerchantID), req.PayType, 2, channelDetail.Currency, payProductId)
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
			single, err := s.SelectSingleChannel(uint(merchant.MerchantID), req.PayType, 2, channelDetail.Currency)
			if err != nil {
				return resp, errors.New("no single channel available")
			}
			// 检查金额是否在通道允许范围内
			orderRange := fmt.Sprintf("%v-%v", single.MinAmount, single.MaxAmount)
			if !utils.MatchOrderRange(amount, orderRange) {
				return resp, errors.New(fmt.Sprintf("the amount does not meet the risk control requirements.order amount:%v,limit amount:%s", amount, orderRange)) // 金额不符合风控要求，跳过
			}
			products = []dto.PayProductVo{single}
		} else {
			products, err = s.selectPollingChannels(uint(merchant.MerchantID), req.PayType, 2, channelDetail.Currency, amount)
			if err != nil {
				return resp, err
			}
		}
	}

	// 8 幂等检查
	oid, exists, err := s.checkIdempotency(merchant.MerchantID, req.TranFlow)
	if err != nil {
		return resp, err
	}
	if exists {
		return resp, nil
	}

	// 9 计算结算
	settle, err := s.calculateSettlement(merchant, products[0], amount)
	if err != nil {
		return resp, err
	}

	// 10 创建订单
	now := time.Now()
	order, tx, err := s.createOrderAndTransaction(merchant, req, products[0], amount, oid, now, settle)
	if err != nil {
		return resp, err
	}

	// 11 调用上游（失败降级）
	var lastErr error
	for _, product := range products {
		_, err = s.callUpstreamService(merchant, &req, &product, tx.UpOrderId)
		if err == nil {
			s.clearUpstreamFail(uint64(product.UpstreamId))
			lastErr = nil
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
		notify.Notify(system.BotChatID, "warn", "代付上游调用失败",
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

	if lastErr != nil {
		resp = dto.CreatePayoutOrderResp{
			PaySerialNo: strconv.FormatUint(oid, 10),
			TranFlow:    req.TranFlow, SysTime: time.Now().Format(time.RFC3339),
			Amount: req.Amount, Code: "001",
		}
		return resp, lastErr
	}

	// 12 构建响应
	resp = dto.CreatePayoutOrderResp{
		PaySerialNo: strconv.FormatUint(oid, 10),
		TranFlow:    req.TranFlow,
		SysTime:     strconv.FormatInt(utils.GetTimestampMs(), 10),
		Amount:      req.Amount, Code: "0", Status: "0001",
	}

	// 13 异步事件
	go s.asyncPostOrderCreation(oid, order, merchant.MerchantID, req.TranFlow, req.Amount, now)
	return resp, nil
}

// asyncPostOrderCreation 异步处理订单创建后的操作
func (s *PayoutOrderService) asyncPostOrderCreation(oid uint64, order *ordermodel.MerchantPayOutOrderM, merchantID uint64, tranFlow, amount string, now time.Time) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[PANIC] asyncPostOrderCreation recovered: %v\n stack: %s", r, debug.Stack())
		}
	}()

	// 缓存到 Redis
	cacheKey := "payout_order:" + strconv.FormatUint(oid, 10)
	orderJSON := utils.MapToJSON(order)
	if orderJSON == "" {
		log.Printf("订单JSON序列化失败: oid=%d", oid)
		return
	}

	if err := dal.RedisClient.Set(dal.RedisCtx, cacheKey, orderJSON, 10*time.Minute).Err(); err != nil {
		log.Printf("缓存订单失败: %v", err)
	}
}

// callUpstreamService 调用上游服务（带防重保护）
func (s *PayoutOrderService) callUpstreamService(
	merchant *mainmodel.Merchant,
	req *dto.CreatePayoutOrderReq,
	payChannelProduct *dto.PayProductVo,
	txId uint64,
) (string, error) {
	// 空指针检查
	if payChannelProduct == nil {
		return "", errors.New("pay product cannot be nil")
	}
	if merchant == nil {
		return "", errors.New("merchant cannot be nil")
	}
	if req == nil {
		return "", errors.New("request cannot be nil")
	}

	// 使用 singleflight 防止重复调用上游
	key := fmt.Sprintf("upstream:%s:%d", req.TranFlow, txId)
	result, err, _ := s.upstreamGroup.Do(key, func() (interface{}, error) {
		return s.callUpstreamServiceInternal(merchant, req, payChannelProduct, txId)
	})

	if err != nil {
		return "", err
	}
	return result.(string), nil
}

// callUpstreamServiceInternal 调用上游服务内部实现
func (s *PayoutOrderService) callUpstreamServiceInternal(
	merchant *mainmodel.Merchant,
	req *dto.CreatePayoutOrderReq,
	payChannelProduct *dto.PayProductVo,
	txId uint64,
) (string, error) {
	var upstreamRequest dto.UpstreamRequest
	upstreamRequest.Currency = payChannelProduct.Currency
	upstreamRequest.Amount = req.Amount
	upstreamRequest.PayType = req.PayType
	upstreamRequest.ProviderKey = payChannelProduct.InterfaceCode
	upstreamRequest.MchOrderId = strconv.FormatUint(txId, 10)
	upstreamRequest.ApiKey = merchant.ApiKey
	upstreamRequest.MchNo = payChannelProduct.UpAccount
	upstreamRequest.NotifyUrl = req.NotifyUrl
	upstreamRequest.IdentityType = req.IdentityType
	upstreamRequest.IdentityNum = req.IdentityNum
	upstreamRequest.PayMethod = req.PayMethod
	upstreamRequest.AccName = req.AccName
	upstreamRequest.AccNo = req.AccNo
	upstreamRequest.BankName = req.BankName
	upstreamRequest.BankCode = req.BankCode
	upstreamRequest.UpstreamCode = payChannelProduct.UpstreamCode
	upstreamRequest.QueryUrl = payChannelProduct.PayoutQueryApi
	upstreamRequest.SubmitUrl = payChannelProduct.PayoutApi
	upstreamRequest.Mode = "payout"

	// 使用带超时的上下文
	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
	defer cancel()

	// 调用上游服务
	mOrderId, upOrderNo, _, err := CallUpstreamPayoutService(ctx, upstreamRequest)
	if err != nil {
		return "", err
	}

	// 更新上游交易订单信息
	if mOrderId != "" {
		mOrderIdUint, err := strconv.ParseUint(mOrderId, 10, 64)
		if err != nil {
			log.Printf("上游订单号转换失败: %v", err)
		} else {
			txTable := shard.UpOutOrderShard.GetTable(txId, time.Now())
			upTx := dto.UpdateUpTxVo{
				UpOrderId: mOrderIdUint,
				UpOrderNo: upOrderNo,
			}

			if err := s.orderDao.UpdateUpTx(txTable, upTx); err != nil {
				log.Printf("更新上游交易失败: %v", err)
			}
		}
	}

	return upOrderNo, nil
}

// createOrderAndTransaction 创建订单和事务
func (s *PayoutOrderService) createOrderAndTransaction(
	merchant *mainmodel.Merchant,
	req dto.CreatePayoutOrderReq,
	payChannelProduct dto.PayProductVo,
	amount decimal.Decimal,
	oid uint64,
	now time.Time,
	settle dto.SettlementResult,
) (*ordermodel.MerchantPayOutOrderM, *ordermodel.PayoutUpstreamTxM, error) {
	var order *ordermodel.MerchantPayOutOrderM
	var tx *ordermodel.PayoutUpstreamTxM

	err := dal.OrderDB.Transaction(func(txDB *gorm.DB) error {
		// 事务内的 dao
		orderDao := dao.NewPayoutOrderDaoWithDB(txDB)

		// 创建订单
		if err := s.createOrder(merchant, req, payChannelProduct, amount, oid, now, settle, orderDao); err != nil {
			return fmt.Errorf("create order failed: %w", err)
		}

		// 创建上游事务
		upTx, err := s.createUpstreamTx(merchant, req, payChannelProduct, oid, now, orderDao)
		if err != nil {
			return fmt.Errorf("create upstream transaction failed: %w", err)
		}
		tx = upTx

		// 创建索引
		if err := s.createOrderIndex(merchant, req, oid, now, orderDao); err != nil {
			return fmt.Errorf("create order index failed: %w", err)
		}
		// 冻结商户资金
		freezeErr := s.freezePayout(merchant.MerchantID, payChannelProduct.Currency, strconv.FormatUint(oid, 10), amount, merchant.NickName)
		if freezeErr != nil {
			return fmt.Errorf("freeze merchant money failed: %w", freezeErr)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	// 查询订单和上游事务 - 添加空指针检查
	orderTable := shard.OutOrderShard.GetTable(oid, now)
	order, err = s.orderDao.GetByOrderId(orderTable, oid)
	if err != nil {
		return nil, nil, fmt.Errorf("get order failed: %w", err)
	}
	if order == nil {
		return nil, nil, errors.New("order not found after creation")
	}

	txTable := shard.UpOutOrderShard.GetTable(tx.UpOrderId, now)
	tx, err = s.orderDao.GetTxByOrderId(txTable, oid)
	if err != nil {
		return nil, nil, fmt.Errorf("get transaction failed: %w", err)
	}
	if tx == nil {
		return nil, nil, errors.New("transaction not found after creation")
	}

	return order, tx, nil
}

// 冻结资金
func (s *PayoutOrderService) freezePayout(uid uint64, currency string, orderNo string, amount decimal.Decimal, operator string) error {

	err := s.mainDao.FreezePayout(uid, currency, orderNo, amount, operator)
	if err != nil {
		return fmt.Errorf("freeze merchant money failed: %w", err)
	}
	return nil
}

// createOrder 创建订单
func (s *PayoutOrderService) createOrder(
	merchant *mainmodel.Merchant,
	req dto.CreatePayoutOrderReq,
	payChannelProduct dto.PayProductVo,
	amount decimal.Decimal,
	oid uint64,
	now time.Time,
	settle dto.SettlementResult,
	orderDao *dao.PayoutOrderDao,
) error {
	var orderSettle dto.SettlementResult
	if err := copier.Copy(&orderSettle, &settle); err != nil {
		return fmt.Errorf("copy settlement failed: %w", err)
	}

	log.Printf(">>>支付产品信息:%+v", payChannelProduct)
	costFee := amount.Mul(payChannelProduct.CostRate).Div(decimal.NewFromInt(100))      //上游成本费用
	orderFee := amount.Mul(payChannelProduct.MDefaultRate).Div(decimal.NewFromInt(100)) //商户手续费
	profitFee := orderFee.Sub(costFee)
	m := &ordermodel.MerchantPayOutOrderM{
		OrderID:        oid,
		MID:            merchant.MerchantID,
		MOrderID:       req.TranFlow,
		Amount:         amount,
		Currency:       payChannelProduct.Currency,
		SupplierID:     payChannelProduct.UpstreamId,
		Status:         1,
		NotifyURL:      req.NotifyUrl,
		ChannelID:      payChannelProduct.SysChannelID,
		UpChannelID:    payChannelProduct.ID,
		ChannelCode:    &payChannelProduct.SysChannelCode,
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
		SettleSnapshot: ordermodel.PayoutSettleSnapshot(orderSettle),
		AID: func() uint64 {
			if merchant.PId > 0 {
				return merchant.PId
			}
			return 0
		}(),
	}

	table := shard.OutOrderShard.GetTable(oid, now)
	if err := orderDao.Insert(table, m); err != nil {
		return fmt.Errorf("insert order failed: %w", err)
	}

	return nil
}

// calculateSettlement 计算结算费用
func (s *PayoutOrderService) calculateSettlement(merchant *mainmodel.Merchant, payChannelProduct dto.PayProductVo, amount decimal.Decimal) (dto.SettlementResult, error) {
	// 验证输入参数
	if merchant == nil {
		return dto.SettlementResult{}, errors.New("merchant cannot be nil")
	}
	if amount.LessThanOrEqual(decimal.Zero) {
		return dto.SettlementResult{}, errors.New("amount must be positive")
	}

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
		if err != nil {
			log.Printf("get agent merchant failed: %v", err)
			// 不返回错误，继续使用零值
		} else if agentInfo != nil && agentInfo.Status == 1 {
			agentPct = agentInfo.DefaultRate
			agentFixed = agentInfo.SingleFee
		}
	}

	// 验证费率有效性
	if payChannelProduct.MDefaultRate.IsNegative() || payChannelProduct.CostRate.IsNegative() {
		return dto.SettlementResult{}, errors.New("invalid rate value")
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

// createUpstreamTx 创建上游事务
func (s *PayoutOrderService) createUpstreamTx(
	merchant *mainmodel.Merchant,
	req dto.CreatePayoutOrderReq,
	payChannelProduct dto.PayProductVo,
	oid uint64,
	now time.Time,
	orderDao *dao.PayoutOrderDao,
) (*ordermodel.PayoutUpstreamTxM, error) {
	txId := idgen.New()
	txTable := shard.UpOutOrderShard.GetTable(txId, now)

	tx := &ordermodel.PayoutUpstreamTxM{
		OrderID:    oid,
		MerchantID: strconv.FormatUint(merchant.MerchantID, 10),
		SupplierId: uint64(payChannelProduct.UpstreamId),
		Amount:     utils.MustStringToDecimal(req.Amount),
		Currency:   payChannelProduct.Currency,
		Status:     0,
		UpOrderId:  txId,
		CreateTime: &now,
	}

	if err := orderDao.InsertTx(txTable, tx); err != nil {
		return nil, fmt.Errorf("insert transaction failed: %w", err)
	}

	// 更新订单表
	updateOrder := dto.UpdateOrderVo{
		OrderId:    oid,
		UpOrderId:  txId,
		UpdateTime: now,
	}

	orderTable := shard.OutOrderShard.GetTable(oid, now)
	if err := orderDao.UpdateOrder(orderTable, updateOrder); err != nil {
		return nil, fmt.Errorf("update order failed: %w", err)
	}

	return tx, nil
}

// createOrderIndex 创建订单索引
func (s *PayoutOrderService) createOrderIndex(
	merchant *mainmodel.Merchant,
	req dto.CreatePayoutOrderReq,
	oid uint64,
	now time.Time,
	orderDao *dao.PayoutOrderDao,
) error {
	receiveIndexTable := utils.GetOrderIndexTable("p_out_order_index", now)
	orderLogIndexTable := shard.OutOrderLogShard.GetTable(oid, now)
	receiveLogIndexTable := shard.OutOrderShard.GetTable(oid, now)

	receiveIndex := &ordermodel.PayoutOrderIndexM{
		MID:               merchant.MerchantID,
		MOrderID:          req.TranFlow,
		OrderID:           oid,
		OrderTableName:    receiveLogIndexTable,
		OrderLogTableName: orderLogIndexTable,
		CreateTime:        now,
	}

	if err := orderDao.InsertPayoutOrderIndexTable(receiveIndexTable, receiveIndex); err != nil {
		return fmt.Errorf("insert order index failed: %w", err)
	}

	return nil
}

// checkIdempotency 检查幂等性
func (s *PayoutOrderService) checkIdempotency(merchantID uint64, tranFlow string) (uint64, bool, error) {
	oid := idgen.New()
	table := shard.OutOrderShard.GetTable(oid, time.Now())

	// 检查是否已存在订单
	exist, err := s.orderDao.GetByMerchantNo(table, merchantID, tranFlow)
	if err != nil {
		return 0, false, fmt.Errorf("check idempotency failed: %w", err)
	}
	if exist != nil {
		return 0, true, nil
	}

	return oid, false, nil
}

// getSysChannelWithCache 获取系统通道信息（带缓存）
func (s *PayoutOrderService) getSysChannelWithCache(channelCode string) (*dto.PayWayVo, error) {
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
			return nil, fmt.Errorf("get sys channel failed: %w", err)
		}
		if channel == nil {
			return nil, errors.New("channel not found")
		}

		// 缓存结果
		channelJSON := utils.MapToJSON(channel)
		if channelJSON != "" {
			dal.RedisClient.Set(dal.RedisCtx, cacheKey, channelJSON, 10*time.Minute)
		}

		return channel, nil
	})

	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, errors.New("channel not found")
	}

	return result.(*dto.PayWayVo), nil
}

// getMerchantWithCache 获取商户信息（带缓存和防击穿）
func (s *PayoutOrderService) getMerchantWithCache(merchantNo string) (*mainmodel.Merchant, error) {
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
		if err != nil {
			return nil, fmt.Errorf("get merchant failed: %w", err)
		}
		if merchant == nil || merchant.Status != 1 {
			return nil, errors.New("merchant not found or invalid")
		}

		// 缓存结果
		merchantJSON := utils.MapToJSON(merchant)
		if merchantJSON != "" {
			dal.RedisClient.Set(dal.RedisCtx, cacheKey, merchantJSON, 5*time.Minute)
		}

		return merchant, nil
	})

	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, errors.New("merchant not found")
	}

	return result.(*mainmodel.Merchant), nil
}

// validateCreatePayoutRequest 验证创建订单请求
func validateCreatePayoutRequest(req dto.CreatePayoutOrderReq) error {
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

func (s *PayoutOrderService) Get(param dto.QueryPayoutOrderReq) (dto.QueryPayoutOrderResp, error) {
	var resp dto.QueryPayoutOrderResp

	indexTable := utils.GetOrderIndexTable("p_out_order_index", time.Now())
	mId, err := s.GetMerchantInfo(param.MerchantNo)
	if err != nil {
		return resp, err
	}

	indexTableResult, err := s.indexTableDao.GetByOutIndexTable(indexTable, param.TranFlow, mId)
	if err != nil {
		return resp, err
	}
	if indexTableResult == nil {
		return resp, errors.New("order index not found")
	}

	orderIndexTable := shard.OutOrderShard.GetTable(indexTableResult.OrderID, time.Now())
	orderData, err := s.orderDao.GetByOrderId(orderIndexTable, indexTableResult.OrderID)
	if err != nil {
		return resp, err
	}
	if orderData == nil {
		return resp, errors.New("order not found")
	}

	resp.Status = utils.ConvertOrderStatus(orderData.Status)
	resp.TranFlow = orderData.MOrderID
	resp.PaySerialNo = strconv.FormatUint(orderData.OrderID, 10)
	resp.Amount = orderData.Amount.String()
	resp.Code = "0"

	return resp, nil
}

// TestSelectSingleChannel 查询单独支付通道
func (s *PayoutOrderService) TestSelectSingleChannel(mId uint, sysChannelCode string, channelType int8, currency string, payProductId uint64) (dto.PayProductVo, error) {
	// 查询单独支付通道产品
	payDetail, err := s.mainDao.GetTestSinglePayChannel(mId, sysChannelCode, channelType, currency, payProductId)

	if err != nil {
		return dto.PayProductVo{}, fmt.Errorf("get single pay channel failed: %w", err)
	}

	return payDetail, nil
}

// SelectSingleChannel 查询单独支付通道
func (s *PayoutOrderService) SelectSingleChannel(mId uint, sysChannelCode string, channelType int8, currency string) (dto.PayProductVo, error) {
	// 查询单独支付通道产品
	payDetail, err := s.mainDao.GetSinglePayChannel(mId, sysChannelCode, channelType, currency)

	if err != nil {
		return dto.PayProductVo{}, fmt.Errorf("get single pay channel failed: %w", err)
	}

	return payDetail, nil
}

// getHealthManager 获取通道健康管理器
func (s *PayoutOrderService) getHealthManager() *health.ChannelHealthManager {
	return &health.ChannelHealthManager{
		Redis:     dal.RedisClient,
		Strategy:  &health.DecayStrategy{Factor: 0.95},
		Threshold: 60.0,
		TTL:       30 * time.Minute,
	}
}

// selectPollingChannelWithRetry 带重试的轮询通道选择
func (s *PayoutOrderService) selectPollingChannelWithRetry(mId uint, sysChannelCode string, channelType int8, currency string, amount decimal.Decimal) (dto.PayProductVo, error) {
	// 获取健康管理器
	healthManager := s.getHealthManager()

	// 获取可用通道产品
	products, err := s.mainDao.GetAvailablePollingPayProducts(mId, sysChannelCode, currency, channelType)
	if err != nil {
		return dto.PayProductVo{}, fmt.Errorf("get available polling products failed: %w", err)
	}
	if len(products) == 0 {
		return dto.PayProductVo{}, errors.New("no channel products available")
	}

	// 按权重降序排序
	sort.SliceStable(products, func(i, j int) bool {
		return products[i].UpstreamWeight > products[j].UpstreamWeight
	})

	// 尝试找到合适的通道
	for _, product := range products {
		// 跳过禁用的通道
		if healthManager.IsDisabled(product.ID) {
			continue
		}

		// 检查费率
		if product.MDefaultRate.LessThanOrEqual(product.CostRate) {
			continue
		}

		// 检查金额范围
		orderRange := fmt.Sprintf("%v-%v", product.MinAmount, product.MaxAmount)
		if !utils.MatchOrderRange(amount, orderRange) {
			continue
		}

		return product, nil
	}

	return dto.PayProductVo{}, errors.New("no suitable channel found after filtering")
}

// SelectPollingChannel 查询轮询所有支付通道
func (s *PayoutOrderService) SelectPollingChannel(mId uint, sysChannelCode string, channelType int8, currency string, amount decimal.Decimal) ([]dto.PayProductVo, error) {
	// 查询所有可用通道产品（状态开启），按 weight 降序
	products, err := s.mainDao.GetAvailablePollingPayProducts(mId, sysChannelCode, currency, channelType)
	if err != nil {
		return nil, fmt.Errorf("get available polling products failed: %w", err)
	}
	if len(products) == 0 {
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

// SelectPaymentChannel 根据商户和订单金额选择可用通道（返回指针，调用方需判空）
func (s *PayoutOrderService) SelectPaymentChannel(merchantID uint, amount decimal.Decimal, channelCode string, currency string) (*dto.PaymentChannelVo, error) {
	mainDao := &dao.MainDao{}
	payRouteList, err := mainDao.SelectPaymentChannel(merchantID, channelCode, currency)
	if err != nil {
		return nil, fmt.Errorf("select payment channel failed: %w", err)
	}
	if len(payRouteList) < 1 {
		return nil, errors.New("没有可用通道")
	}

	for _, route := range payRouteList {
		if utils.MatchOrderRange(amount, route.OrderRange) {
			// 返回地址拷贝，防止外部修改底层切片数据
			r := route
			return &r, nil
		}
	}

	return nil, fmt.Errorf("no available payment channel")
}

// QuerySysChannel 查询系统通道编码（返回指针）
func (s *PayoutOrderService) QuerySysChannel(channelCode string) (*dto.PayWayVo, error) {
	mainDao := &dao.MainDao{}
	payWayDetail, err := mainDao.GetSysChannel(channelCode)
	if err != nil {
		return nil, fmt.Errorf("get sys channel failed: %w", err)
	}
	if payWayDetail == nil {
		return nil, errors.New("通道编码不存在")
	}
	return payWayDetail, nil
}

func (s *PayoutOrderService) GetMerchantInfo(appId string) (uint64, error) {
	merchant, err := s.mainDao.GetMerchant(appId)
	if err != nil {
		return 0, fmt.Errorf("get merchant failed: %w", err)
	}
	if merchant == nil || merchant.Status != 1 {
		return 0, errors.New("merchant invalid")
	}
	return merchant.MerchantID, nil
}

// HealthCheck 服务健康检查
func (s *PayoutOrderService) HealthCheck() bool {
	s.healthCheckLock.Lock()
	defer s.healthCheckLock.Unlock()

	// 避免频繁检查，至少间隔5秒
	if time.Since(s.lastHealthCheck) < 5*time.Second && s.isHealthy {
		return true
	}

	s.lastHealthCheck = time.Now()

	// 检查数据库连接
	ctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
	defer cancel()

	if err := dal.OrderDB.WithContext(ctx).Exec("SELECT 1").Error; err != nil {
		log.Printf("数据库健康检查失败: %v", err)
		s.isHealthy = false
		return false
	}

	// 检查 Redis 连接
	if err := dal.RedisClient.Ping(dal.RedisCtx).Err(); err != nil {
		log.Printf("Redis健康检查失败: %v", err)
		s.isHealthy = false
		return false
	}

	s.isHealthy = true
	return true
}

// IsHealthy 返回服务健康状态
func (s *PayoutOrderService) IsHealthy() bool {
	s.healthCheckLock.RLock()
	defer s.healthCheckLock.RUnlock()
	return s.isHealthy
}

// InitializePayoutService 初始化支付服务
func InitializePayoutService() (*PayoutOrderService, error) {
	service := NewPayoutOrderService()

	if !service.IsHealthy() {
		return nil, errors.New("service health check failed")
	}

	log.Println("PayoutOrderService 初始化成功")
	return service, nil
}
