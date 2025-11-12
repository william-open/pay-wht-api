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

const reassignUpstreamFailKey = "reassign_up_fail:"

type ReassignOrderService struct {
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

func NewReassignOrderService() *ReassignOrderService {
	ctx, cancel := context.WithCancel(context.Background())
	service := &ReassignOrderService{
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
		log.Println("ReassignOrderService 初始化成功")
	} else {
		log.Println("ReassignOrderService 初始化警告：健康检查未通过")
	}

	return service
}

func (s *ReassignOrderService) Shutdown() {
	s.cancel()
}

// ================== 上游失败监控（多维度统计） ==================
func (s *ReassignOrderService) recordUpstreamFail(upstreamID uint64, upstreamName, upstreamCode, sysChannelCode string) {
	key := fmt.Sprintf("%s%d:%s:%s", reassignUpstreamFailKey, upstreamID, upstreamCode, sysChannelCode)
	cnt, _ := dal.RedisClient.Incr(dal.RedisCtx, key).Result()
	if cnt == 1 {
		dal.RedisClient.Expire(dal.RedisCtx, key, 5*time.Minute)
	}

	// ⚠️ 第3次警告
	if cnt == 3 {
		notify.Notify(system.BotChatID, "warn", "改派通道降权提醒",
			fmt.Sprintf(
				"⚠️ 改派通道失败提醒\n上游供应商名称: *%s*\n上游供应商ID: `%d`\n上游供应商通道编码: `%s`\n系统通道编码: `%s`\n\n5分钟内失败 ≥3 次，权重减半。",
				upstreamName, upstreamID, upstreamCode, sysChannelCode,
			), false)
	}

	// 🚨 第10次告警
	if cnt >= 10 {
		notify.Notify(system.BotChatID, "error", "改派通道严重告警",
			fmt.Sprintf(
				"🚨 改派通道连续失败\n上游供应商名称: *%s*\n上游供应商ID: `%d`\n上游供应商通道编码: `%s`\n系统通道编码: `%s`\n\n5分钟内失败次数已达 `%d` 次！",
				upstreamName, upstreamID, upstreamCode, sysChannelCode, cnt,
			), true)
	}
}

// 清理失败计数
func (s *ReassignOrderService) clearUpstreamFail(upstreamID uint64, upstreamCode, sysChannelCode string) {
	key := fmt.Sprintf("%s%d:%s:%s", reassignUpstreamFailKey, upstreamID, upstreamCode, sysChannelCode)
	dal.RedisClient.Del(dal.RedisCtx, key)
}

// 获取失败次数
func (s *ReassignOrderService) getUpstreamFailCount(upstreamID uint64, upstreamCode, sysChannelCode string) int {
	key := fmt.Sprintf("%s%d:%s:%s", reassignUpstreamFailKey, upstreamID, upstreamCode, sysChannelCode)
	val, _ := dal.RedisClient.Get(dal.RedisCtx, key).Result()
	if val == "" {
		return 0
	}
	cnt, _ := strconv.Atoi(val)
	return cnt
}

// ================== 轮询通道选择 ==================
func (s *ReassignOrderService) selectPollingChannels(
	merchantID uint, sysChannelCode string, channelType int8, currency string, amount decimal.Decimal,
) ([]dto.PayProductVo, error) {
	products, err := s.mainDao.GetAvailablePollingPayProducts(merchantID, sysChannelCode, currency, channelType)
	if err != nil || len(products) == 0 {
		return nil, errors.New("no channel products available")
	}
	for i := range products {
		failCnt := s.getUpstreamFailCount(
			uint64(products[i].UpstreamId),
			products[i].UpstreamCode,
			products[i].SysChannelCode,
		)
		if failCnt >= 3 {
			products[i].UpstreamWeight = max(1, products[i].UpstreamWeight/2)
		}
	}
	sort.SliceStable(products, func(i, j int) bool {
		return products[i].UpstreamWeight > products[j].UpstreamWeight
	})
	return products, nil
}

// Create 处理代付改派订单逻辑（优化版：仅调用一次指定上游 + 成功后修正订单）
func (s *ReassignOrderService) Create(req dto.CreateReassignOrderReq) (resp dto.CreateReassignOrderResp, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[PANIC] reassign order Create panic: %v\n%s", r, debug.Stack())
			notify.Notify(system.BotChatID, "error", "代付改派Panic", fmt.Sprintf("panic: %v", r), true)
			resp = dto.CreateReassignOrderResp{
				PaySerialNo: "", TranFlow: req.TranFlow, SysTime: time.Now().Format(time.RFC3339),
				Amount: req.Amount, Code: "999",
			}
			err = fmt.Errorf("internal error")
		}
	}()

	// 健康检查
	if !s.IsHealthy() {
		return resp, errors.New("service temporarily unavailable")
	}

	// 1 参数验证
	if err := validateCreateReassignRequest(req); err != nil {
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
		return resp, errors.New("order amount invalid")
	}

	// 4 系统通道
	channelDetail, err := s.getSysChannelWithCache(req.PayType)
	if err != nil || channelDetail == nil {
		return resp, errors.New("system channel invalid")
	}

	// 5 商户通道
	merchantChannelInfo, err := NewCommonService().GetMerchantChannelInfo(merchant.MerchantID, req.PayType)
	if err != nil || merchantChannelInfo == nil {
		return resp, fmt.Errorf("merchant channel invalid, payType: %s", req.PayType)
	}

	// 6 指定上游通道
	if req.PayProductId == "" {
		return resp, fmt.Errorf("merchant specify upstream channel empty, payType: %s", req.PayType)
	}
	payProductId, err := strconv.ParseUint(req.PayProductId, 10, 64)
	if err != nil {
		return resp, errors.New("admin reassign pay_product_id parse error")
	}

	single, err := s.TestSelectSingleChannel(uint(merchant.MerchantID), req.PayType, 2, channelDetail.Currency, payProductId)
	if err != nil {
		return resp, errors.New("no single channel available")
	}

	orderRange := fmt.Sprintf("%v-%v", single.MinAmount, single.MaxAmount)
	if !utils.MatchOrderRange(amount, orderRange) {
		return resp, fmt.Errorf(
			"order amount does not meet risk control requirements: order=%v, limit=%s",
			amount, orderRange,
		)
	}

	orderId, err := strconv.ParseUint(req.OrderId, 10, 64)
	if err != nil {
		return resp, fmt.Errorf("invalid orderId: %v", err)
	}
	if req.OrderId == "" {
		return resp, errors.New("reassign order failed, orderId is empty")
	}

	// 7 幂等检查
	_, exists, err := s.checkIdempotency(merchant.MerchantID, req.TranFlow, orderId)
	if err != nil {
		return resp, err
	}
	if !exists {
		return resp, nil
	}

	// 8 计算结算
	settle, err := s.calculateSettlement(merchant, single, amount)
	if err != nil {
		return resp, err
	}

	// 9 商户余额
	merchantMoney, mmErr := s.mainDao.GetMerchantAccount(strconv.FormatUint(merchant.MerchantID, 10))
	if mmErr != nil || merchantMoney.Money.LessThan(amount.Add(settle.AgentTotalFee).Add(settle.MerchantTotalFee)) {
		return resp, errors.New("merchant insufficient balance")
	}

	// 10 创建订单
	now := time.Now()
	order, _, err := s.createTransaction(merchant, req, single, orderId, now, settle)
	if err != nil {
		return resp, err
	}

	// 11 调用上游（仅一次）
	singleProduct := single
	var lastErr error

	_, err = s.callUpstreamService(merchant, &req, &singleProduct, order)
	if err != nil {
		// 失败
		s.recordUpstreamFail(
			uint64(singleProduct.UpstreamId),
			singleProduct.UpstreamTitle,
			singleProduct.UpstreamCode,
			singleProduct.SysChannelCode,
		)
		go func(pid int64) {
			if e := s.mainDao.UpdateSuccessRate(pid, false); e != nil {
				log.Printf("update channel success rate failed: %v", e)
			}
		}(singleProduct.ID)

		notify.Notify(system.BotChatID, "warn", "改派代付上游调用失败",
			fmt.Sprintf(
				"\n商户号: %s\n通道编码: %s\n上游通道: %s\n上游接口: %s\n供应商: %s\n订单号: %s\n失败原因: %v\n商户请求参数: %s",
				req.MerchantNo,
				req.PayType,
				singleProduct.UpChannelTitle,
				singleProduct.InterfaceCode,
				singleProduct.UpstreamTitle,
				req.TranFlow,
				err,
				utils.MapToJSON(req),
			),
			true,
		)
		lastErr = err
	} else {
		// 成功
		s.clearUpstreamFail(
			uint64(singleProduct.UpstreamId),
			singleProduct.UpstreamCode,
			singleProduct.SysChannelCode,
		)
		lastErr = nil
		go func(pid int64) {
			if e := s.mainDao.UpdateSuccessRate(pid, true); e != nil {
				log.Printf("update channel success rate failed: %v", e)
			}
		}(singleProduct.ID)

		// ✅ 改派成功后修正订单表信息
		if uErr := s.updateReassignOrderInfo(order, merchant, singleProduct, settle, amount); uErr != nil {
			log.Printf("[WARN] 改派订单更新失败: %v", uErr)
			notify.Notify(system.BotChatID, "warn", "改派订单更新失败",
				fmt.Sprintf("商户号:%s\n订单号:%s\n原因:%v", req.MerchantNo, req.TranFlow, uErr), true)
		}
	}

	// 12 响应构建
	if lastErr != nil {
		resp = dto.CreateReassignOrderResp{
			PaySerialNo: strconv.FormatUint(orderId, 10),
			TranFlow:    req.TranFlow,
			SysTime:     time.Now().Format(time.RFC3339),
			Amount:      req.Amount,
			Code:        "001",
		}
		return resp, lastErr
	}

	resp = dto.CreateReassignOrderResp{
		PaySerialNo: strconv.FormatUint(orderId, 10),
		TranFlow:    req.TranFlow,
		SysTime:     strconv.FormatInt(utils.GetTimestampMs(), 10),
		Amount:      req.Amount,
		Code:        "0",
		Status:      "0001",
	}

	// 13 异步缓存
	go s.asyncPostOrderCreation(orderId, order, merchant.MerchantID, req.TranFlow, req.Amount, now)
	return resp, nil
}

// updateReassignOrderInfo 改派成功后更新订单通道、费率、分润、结算信息并调整冻结金额
func (s *ReassignOrderService) updateReassignOrderInfo(
	order *ordermodel.MerchantPayOutOrderM,
	merchant *mainmodel.Merchant,
	product dto.PayProductVo,
	settle dto.SettlementResult,
	amount decimal.Decimal,
) error {
	if order == nil {
		return errors.New("order cannot be nil")
	}

	now := time.Now()
	orderTable := shard.OutOrderShard.GetTable(order.OrderID, now)

	// ==================== 1️⃣ 重新计算费率与利润 ====================
	costFee := amount.Mul(product.CostRate).Div(decimal.NewFromInt(100)).Add(product.CostFee)
	orderFee := amount.Mul(product.MDefaultRate).Div(decimal.NewFromInt(100)).Add(product.MSingleFee)
	profit := orderFee.Sub(costFee)

	// ==================== 2️⃣ 计算新冻结金额 ====================
	newFreezeAmount := amount.Add(settle.AgentTotalFee).Add(settle.MerchantTotalFee)
	oldFreeze := order.FreezeAmount
	diff := newFreezeAmount.Sub(oldFreeze)

	// ==================== 3️⃣ 差额补冻结逻辑 ====================
	if diff.GreaterThan(decimal.Zero) {
		log.Printf("[REASSIGN-FREEZE-ADJUST] 检测到改派通道冻结金额更高，补冻结差额: %s (旧=%s, 新=%s)",
			diff.StringFixed(4), oldFreeze.StringFixed(4), newFreezeAmount.StringFixed(4))

		if err := s.mainDao.FreezePayout(
			merchant.MerchantID,
			product.Currency,
			strconv.FormatUint(order.OrderID, 10),
			order.MOrderID,
			diff,
			merchant.NickName,
		); err != nil {
			msg := fmt.Sprintf(
				"⚠️ 改派补冻结失败\n商户ID: `%d`\n订单号: `%s`\n原冻结: `%s`\n新冻结: `%s`\n差额: `%s`\n错误: `%v`",
				merchant.MerchantID,
				order.MOrderID,
				oldFreeze.StringFixed(4),
				newFreezeAmount.StringFixed(4),
				diff.StringFixed(4),
				err,
			)
			log.Printf("[REASSIGN-FREEZE-ADJUST][FAIL] %v", msg)
			notify.Notify(system.BotChatID, "warn", "改派补冻结失败", msg, true)
		} else {
			log.Printf("[REASSIGN-FREEZE-ADJUST] ✅ 成功补冻结 %.4f 元", diff)
			notify.Notify(system.BotChatID, "info", "改派补冻结成功",
				fmt.Sprintf("订单号: `%d`\n补冻结金额: `%s`\n通道: `%s/%s`",
					order.OrderID, diff.StringFixed(4), product.SysChannelCode, product.UpstreamCode), false)
		}
	} else if diff.LessThan(decimal.Zero) {
		// ⚠️ 新通道冻结金额更低：不退差额，只打印日志
		log.Printf("[REASSIGN-FREEZE-ADJUST] 新通道冻结金额更低 (旧=%s, 新=%s)，不退差额。",
			oldFreeze.StringFixed(4), newFreezeAmount.StringFixed(4))
	}

	// ==================== 4️⃣ 更新订单信息 ====================
	updateData := map[string]interface{}{
		"supplier_id":      product.UpstreamId,
		"channel_id":       product.SysChannelID,
		"up_channel_id":    product.ID,
		"channel_code":     product.SysChannelCode,
		"channel_title":    product.SysChannelTitle,
		"up_channel_code":  product.UpstreamCode,
		"up_channel_title": product.UpChannelTitle,
		"m_rate":           product.MDefaultRate,
		"up_rate":          product.CostRate,
		"m_fixed_fee":      product.MSingleFee,
		"up_fixed_fee":     product.CostFee,
		"fees":             settle.MerchantTotalFee,
		"cost":             costFee,
		"profit":           profit,
		"freeze_amount":    newFreezeAmount,
		"settle_snapshot":  ordermodel.PayoutSettleSnapshot(settle),
		"status":           1,
		"remark":           fmt.Sprintf("改派成功→%s/%s %s", product.SysChannelCode, product.UpstreamCode, now.Format("15:04:05")),
		"update_time":      now,
	}

	if err := s.orderDao.UpdateByWhere(orderTable, map[string]interface{}{
		"order_id": order.OrderID,
	}, updateData); err != nil {
		log.Printf("[WARN] 改派订单更新失败: %v", err)
		return fmt.Errorf("update order channel info failed: %w", err)
	}

	log.Printf("[REASSIGN-ORDER-UPDATE] ✅ 改派订单更新成功 order=%d 通道=%s/%s 新冻结=%s",
		order.OrderID, product.SysChannelCode, product.UpstreamCode, newFreezeAmount.StringFixed(4))

	// ==================== 5️⃣ 更新上游交易表 ====================
	txTable := shard.UpOutOrderShard.GetTable(*order.UpOrderID, now)
	updateTx := map[string]interface{}{
		"supplier_id": product.UpstreamId,
		"currency":    product.Currency,
		"update_time": now,
	}
	if err := dal.OrderDB.Table(txTable).
		Where("order_id = ? AND up_order_id = ?", order.OrderID, order.UpOrderID).
		Updates(updateTx).Error; err != nil {
		log.Printf("[WARN] update payout upstream tx failed: %v", err)
	} else {
		log.Printf("[REASSIGN-TX-UPDATE] ✅ 上游交易同步完成 order=%d supplier=%d", order.OrderID, product.UpstreamId)
	}

	return nil
}

// asyncPostOrderCreation 异步处理订单创建后的操作
func (s *ReassignOrderService) asyncPostOrderCreation(oid uint64, order *ordermodel.MerchantPayOutOrderM, merchantID uint64, tranFlow, amount string, now time.Time) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[PANIC] asyncPostOrderCreation recovered: %v\n stack: %s", r, debug.Stack())
		}
	}()

	// 缓存到 Redis
	cacheKey := "payout_reassign_order:" + strconv.FormatUint(oid, 10)
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
func (s *ReassignOrderService) callUpstreamService(
	merchant *mainmodel.Merchant,
	req *dto.CreateReassignOrderReq,
	payChannelProduct *dto.PayProductVo,
	order *ordermodel.MerchantPayOutOrderM,
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
	key := fmt.Sprintf("upstream:%s:%d", req.TranFlow, order.UpOrderID)
	result, err, _ := s.upstreamGroup.Do(key, func() (interface{}, error) {
		return s.callUpstreamServiceInternal(merchant, req, payChannelProduct, order)
	})

	if err != nil {
		return "", err
	}
	return result.(string), nil
}

// callUpstreamServiceInternal 调用上游服务内部实现
func (s *ReassignOrderService) callUpstreamServiceInternal(
	merchant *mainmodel.Merchant,
	req *dto.CreateReassignOrderReq,
	payChannelProduct *dto.PayProductVo,
	order *ordermodel.MerchantPayOutOrderM,
) (string, error) {
	var bankName, bankCode string

	if req.BankCode != "" {
		// 根据接平台银行编码查询平台银行信息
		platformBank, pbErr := s.mainDao.QueryPlatformBankInfo(req.BankCode, merchant.Currency)
		if pbErr != nil {
			return "", fmt.Errorf(fmt.Sprintf("platform Bank code does not exist,%s", req.BankCode))
		}

		// 根据接口ID+平台银行编码+国家货币查询对应上游银行编码+银行名称
		if payChannelProduct.InterfacePayoutVerifyBank > 0 {

			upstreamBank, ubErr := s.mainDao.QueryUpstreamBankInfo(payChannelProduct.InterfaceID, req.BankCode, payChannelProduct.Currency)
			if ubErr != nil {
				return "", fmt.Errorf(fmt.Sprintf("upstream Bank code does not exist,%s", req.BankCode))
			} else {
				bankCode = upstreamBank.UpstreamBankCode
				bankName = upstreamBank.UpstreamBankName
			}
		} else {
			bankCode = platformBank.Code
			bankName = platformBank.Name
		}
	}

	var upstreamRequest dto.UpstreamRequest
	trxMchOrderId := *order.UpOrderID
	upstreamRequest.Currency = payChannelProduct.Currency
	upstreamRequest.Amount = req.Amount
	upstreamRequest.PayType = req.PayType
	upstreamRequest.ProviderKey = payChannelProduct.InterfaceCode
	upstreamRequest.MchOrderId = strconv.FormatUint(trxMchOrderId, 10)
	upstreamRequest.ApiKey = merchant.ApiKey
	upstreamRequest.MchNo = payChannelProduct.UpAccount
	upstreamRequest.NotifyUrl = req.NotifyUrl
	upstreamRequest.IdentityType = req.IdentityType
	upstreamRequest.IdentityNum = req.IdentityNum
	upstreamRequest.PayMethod = req.PayMethod
	upstreamRequest.PayPhone = req.PayPhone
	upstreamRequest.PayEmail = req.PayEmail
	upstreamRequest.AccName = req.AccName
	upstreamRequest.AccNo = req.AccNo
	upstreamRequest.BankName = bankName
	upstreamRequest.BankCode = bankCode
	upstreamRequest.UpstreamCode = payChannelProduct.UpstreamCode
	upstreamRequest.UpstreamTitle = payChannelProduct.UpstreamTitle
	upstreamRequest.QueryUrl = payChannelProduct.PayoutQueryApi
	upstreamRequest.SubmitUrl = payChannelProduct.PayoutApi
	upstreamRequest.Mode = "reassign"
	upstreamRequest.ClientIp = req.ClientId
	upstreamRequest.DownstreamOrderNo = req.TranFlow

	// 使用带超时的上下文
	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
	defer cancel()

	// 调用上游服务
	_, upOrderNo, _, err := CallUpstreamPayoutService(ctx, upstreamRequest, merchant.MerchantID, order)
	if err != nil {
		return "", err
	}

	// 更新上游交易订单信息
	if upOrderNo != "" {
		txTable := shard.UpOutOrderShard.GetTable(trxMchOrderId, time.Now())
		upTx := dto.UpdateUpTxVo{
			UpOrderId: trxMchOrderId,
			UpOrderNo: upOrderNo,
		}

		if err := s.orderDao.UpdateUpTx(txTable, upTx); err != nil {
			log.Printf("更新上游交易失败: %v", err)
		}
	}

	return upOrderNo, nil
}

// createTransaction 创建订单和事务
func (s *ReassignOrderService) createTransaction(
	merchant *mainmodel.Merchant,
	req dto.CreateReassignOrderReq,
	payChannelProduct dto.PayProductVo,
	orderId uint64,
	now time.Time,
	settle dto.SettlementResult,
) (*ordermodel.MerchantPayOutOrderM, *ordermodel.PayoutUpstreamTxM, error) {
	log.Printf(">>创建代付订单，新的交易订单: %v", orderId)
	var order *ordermodel.MerchantPayOutOrderM
	var tx *ordermodel.PayoutUpstreamTxM

	err := dal.OrderDB.Transaction(func(txDB *gorm.DB) error {
		// 事务内的 dao
		orderDao := dao.NewPayoutOrderDaoWithDB(txDB)

		// 创建上游事务
		upTx, err := s.createUpstreamTx(merchant, req, payChannelProduct, orderId, now, orderDao)
		if err != nil {
			return fmt.Errorf("[%v]create upstream transaction failed: %w", orderId, err)
		}
		tx = upTx
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	// 查询订单和上游事务 - 添加空指针检查
	orderTable := shard.OutOrderShard.GetTable(orderId, now)
	order, err = s.orderDao.GetByOrderId(orderTable, orderId)
	if err != nil {
		return nil, nil, fmt.Errorf("get order [%v] failed: %w", orderId, err)
	}
	if order == nil {
		return nil, nil, errors.New("order not found after creation")
	}
	var orderSettle dto.SettlementResult
	if err := copier.Copy(&orderSettle, &settle); err != nil {
		return nil, nil, fmt.Errorf("copy settlement failed: %w", err)
	}

	updateErr := s.orderDao.UpdateByWhere(orderTable, map[string]interface{}{
		"order_id": orderId,
	}, map[string]interface{}{
		"settle_snapshot": ordermodel.PayoutSettleSnapshot(orderSettle),
	})

	if updateErr != nil {
		log.Printf("[WARN] 更新订单结算失败: table=%s, orderId=%d, err=%v", orderTable, orderId, updateErr)
		return nil, nil, fmt.Errorf("reassign order update order settle failed: %w", updateErr)
	}

	txTable := shard.UpOutOrderShard.GetTable(tx.UpOrderId, now)
	tx, err = s.orderDao.GetTxByOrderId(txTable, orderId)
	if err != nil {
		return nil, nil, fmt.Errorf("get transaction failed: %w", err)
	}
	if tx == nil {
		return nil, nil, errors.New("transaction not found after creation")
	}

	return order, tx, nil
}

// 冻结资金
func (s *ReassignOrderService) freezePayout(uid uint64, currency string, orderNo string, mOrderNo string, amount decimal.Decimal, operator string) error {

	err := s.mainDao.FreezePayout(uid, currency, orderNo, mOrderNo, amount, operator)
	if err != nil {
		return fmt.Errorf("freeze merchant money failed: %w", err)
	}
	return nil
}

// createOrder 创建订单
func (s *ReassignOrderService) createOrder(
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
	costFee := amount.Mul(payChannelProduct.CostRate).Div(decimal.NewFromInt(100)) //上游成本费用
	costFee = costFee.Add(payChannelProduct.CostFee)
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
func (s *ReassignOrderService) calculateSettlement(merchant *mainmodel.Merchant, payChannelProduct dto.PayProductVo, amount decimal.Decimal) (dto.SettlementResult, error) {
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
func (s *ReassignOrderService) createUpstreamTx(
	merchant *mainmodel.Merchant,
	req dto.CreateReassignOrderReq,
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
		OrderId:       oid,
		UpOrderId:     txId,
		SupplierId:    uint64(payChannelProduct.UpstreamId),
		ReassignOrder: 1,
		UpdateTime:    now,
	}

	orderTable := shard.OutOrderShard.GetTable(oid, now)
	if err := orderDao.UpdateOrder(orderTable, updateOrder); err != nil {
		return nil, fmt.Errorf("update order failed: %w", err)
	}

	return tx, nil
}

// createOrderIndex 创建订单索引
func (s *ReassignOrderService) createOrderIndex(
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
func (s *ReassignOrderService) checkIdempotency(merchantID uint64, tranFlow string, orderId uint64) (uint64, bool, error) {
	log.Printf("改派订单号:%v", orderId)
	oid := orderId
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
func (s *ReassignOrderService) getSysChannelWithCache(channelCode string) (*dto.PayWayVo, error) {
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
func (s *ReassignOrderService) getMerchantWithCache(merchantNo string) (*mainmodel.Merchant, error) {
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

// validateCreateReassignRequest 验证创建订单请求
func validateCreateReassignRequest(req dto.CreateReassignOrderReq) error {
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

func (s *ReassignOrderService) Get(param dto.QueryPayoutOrderReq) (dto.QueryPayoutOrderResp, error) {
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
func (s *ReassignOrderService) TestSelectSingleChannel(mId uint, sysChannelCode string, channelType int8, currency string, payProductId uint64) (dto.PayProductVo, error) {
	// 查询单独支付通道产品
	payDetail, err := s.mainDao.GetTestSinglePayChannel(mId, sysChannelCode, channelType, currency, payProductId)

	if err != nil {
		return dto.PayProductVo{}, fmt.Errorf("get single pay channel failed: %w", err)
	}

	return payDetail, nil
}

// SelectSingleChannel 查询单独支付通道
func (s *ReassignOrderService) SelectSingleChannel(mId uint, sysChannelCode string, channelType int8, currency string) (dto.PayProductVo, error) {
	// 查询单独支付通道产品
	payDetail, err := s.mainDao.GetSinglePayChannel(mId, sysChannelCode, channelType, currency)

	if err != nil {
		return dto.PayProductVo{}, fmt.Errorf("get single pay channel failed: %w", err)
	}

	return payDetail, nil
}

// getHealthManager 获取通道健康管理器
func (s *ReassignOrderService) getHealthManager() *health.ChannelHealthManager {
	return &health.ChannelHealthManager{
		Redis:     dal.RedisClient,
		Strategy:  &health.DecayStrategy{Factor: 0.95},
		Threshold: 60.0,
		TTL:       30 * time.Minute,
	}
}

// selectPollingChannelWithRetry 带重试的轮询通道选择
func (s *ReassignOrderService) selectPollingChannelWithRetry(mId uint, sysChannelCode string, channelType int8, currency string, amount decimal.Decimal) (dto.PayProductVo, error) {
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
func (s *ReassignOrderService) SelectPollingChannel(mId uint, sysChannelCode string, channelType int8, currency string, amount decimal.Decimal) ([]dto.PayProductVo, error) {
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
func (s *ReassignOrderService) SelectPaymentChannel(merchantID uint, amount decimal.Decimal, channelCode string, currency string) (*dto.PaymentChannelVo, error) {
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
func (s *ReassignOrderService) QuerySysChannel(channelCode string) (*dto.PayWayVo, error) {
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

func (s *ReassignOrderService) GetMerchantInfo(appId string) (uint64, error) {
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
func (s *ReassignOrderService) HealthCheck() bool {
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
func (s *ReassignOrderService) IsHealthy() bool {
	s.healthCheckLock.RLock()
	defer s.healthCheckLock.RUnlock()
	return s.isHealthy
}

// InitializeReassignOrderService 初始化支付服务
func InitializeReassignOrderService() (*ReassignOrderService, error) {
	service := NewReassignOrderService()

	if !service.IsHealthy() {
		return nil, errors.New("service health check failed")
	}

	log.Println("ReassignOrderService 初始化成功")
	return service, nil
}
