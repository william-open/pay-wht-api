package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/jinzhu/copier"
	"gorm.io/gorm"
	"log"
	"math"
	"runtime/debug"
	"sort"
	"strconv"
	"sync"
	"time"
	"wht-order-api/internal/event"
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
	pub           event.Publisher
}

func NewReceiveOrderService(pub event.Publisher) *ReceiveOrderService {
	ctx, cancel := context.WithCancel(context.Background())
	return &ReceiveOrderService{
		mainDao:       dao.NewMainDao(),
		orderDao:      dao.NewOrderDao(), // 默认全局 DB
		indexTableDao: dao.NewIndexTableDao(),
		ctx:           ctx,
		cancel:        cancel,
		pub:           pub, // 注入
	}
}

func (s *ReceiveOrderService) Shutdown() {
	s.cancel()
}

// 记录失败
// 记录失败（带唯一通道维度）
func (s *ReceiveOrderService) recordUpstreamFail(upstreamID uint64, upstreamName, upstreamCode, sysChannelCode string) {
	key := fmt.Sprintf("%s%d:%s:%s", upstreamFailKey, upstreamID, upstreamCode, sysChannelCode)
	cnt, _ := dal.RedisClient.Incr(dal.RedisCtx, key).Result()

	if cnt == 1 {
		dal.RedisClient.Expire(dal.RedisCtx, key, 5*time.Minute)
	}

	// 通知逻辑
	if cnt == 3 {
		notify.Notify(system.BotChatID, "warn", "通道降权提醒",
			fmt.Sprintf("⚠️ 上游通道失败提醒\n上游供应商名称: *%s*\n上游供应商ID: `%d`\n上游供应商通道编码: `%s`\n系统通道编码: `%s`\n\n5分钟内失败 ≥3 次，权重减半。",
				upstreamName, upstreamID, upstreamCode, sysChannelCode), false)
	}

	if cnt >= 10 {
		notify.Notify(system.BotChatID, "error", "上游通道告警",
			fmt.Sprintf("🚨 上游通道连续失败\n上游供应商名称: *%s*\n上游供应商ID: `%d`\n上游供应商通道编码: `%s`\n系统通道编码: `%s`\n\n5分钟内失败次数已达 `%d` 次！",
				upstreamName, upstreamID, upstreamCode, sysChannelCode, cnt), true)
	}
}

// 清理失败计数
func (s *ReceiveOrderService) clearUpstreamFail(upstreamID uint64, upstreamCode, sysChannelCode string) {
	key := fmt.Sprintf("%s%d:%s:%s", upstreamFailKey, upstreamID, upstreamCode, sysChannelCode)
	dal.RedisClient.Del(dal.RedisCtx, key)
}

// 获取失败次数
func (s *ReceiveOrderService) getUpstreamFailCount(upstreamID uint64, upstreamCode, sysChannelCode string) int {
	key := fmt.Sprintf("%s%d:%s:%s", upstreamFailKey, upstreamID, upstreamCode, sysChannelCode)
	val, _ := dal.RedisClient.Get(dal.RedisCtx, key).Result()
	if val == "" {
		return 0
	}
	cnt, _ := strconv.Atoi(val)
	return cnt
}

// -------------------- Create 主流程 --------------------
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

	// 参数验证
	if err = validateCreateRequest(req); err != nil {
		return resp, err
	}

	// 商户信息
	merchant, err := s.getMerchantWithCache(req.MerchantNo)
	if err != nil || merchant == nil {
		return resp, fmt.Errorf("merchant invalid: %w", err)
	}

	// 金额校验
	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		return resp, errors.New("amount format error")
	}

	// 通道信息
	channelDetail, err := s.getSysChannelWithCache(req.PayType)
	if err != nil || channelDetail == nil {
		return resp, errors.New("channel invalid")
	}

	// 商户通道信息
	merchantChannelInfo, err := NewCommonService().GetMerchantChannelInfo(merchant.MerchantID, req.PayType)
	if err != nil || merchantChannelInfo == nil {
		return resp, errors.New("merchant channel invalid")
	}

	// ================== 平滑加权轮询通道选择 ==================
	var products []dto.PayProductVo
	if req.PayProductId != "" {
		payProductId, err := strconv.ParseUint(req.PayProductId, 10, 64)
		if err != nil {
			return resp, errors.New("test admin pay_product_id transfer error")
		}
		single, err := s.TestSelectSingleChannel(uint(merchant.MerchantID), req.PayType, 1, channelDetail.Currency, payProductId)
		if err != nil {
			return resp, errors.New("admin test channel invalid")
		}
		// 检查金额是否在通道允许范围内
		orderRange := fmt.Sprintf("%v-%v", single.MinAmount, single.MaxAmount)
		if !utils.MatchOrderRange(amount, orderRange) {
			return resp, errors.New(fmt.Sprintf("admin test the amount does not meet the risk control requirements.order amount: %v,limit amount: %v", amount, orderRange)) // 金额不符合风控要求，跳过
		}
		products = []dto.PayProductVo{single}
	} else {
		if merchantChannelInfo.DispatchMode == 2 {
			single, err := s.SelectSingleChannel(uint(merchant.MerchantID), req.PayType, 1, channelDetail.Currency)
			if err != nil {
				return resp, err
			}
			products = []dto.PayProductVo{single}
		} else {
			products, err = s.selectWeightedPollingChannel(uint(merchant.MerchantID), req.PayType, 1, channelDetail.Currency, amount)
			if err != nil {
				return resp, err
			}
		}
	}

	// 幂等检查
	oid, exists, err := s.checkIdempotency(merchant.MerchantID, req.TranFlow)
	if err != nil {
		return resp, err
	}
	if exists {
		return resp, nil
	}

	// 结算计算
	settle, err := s.calculateSettlement(merchant, products[0], amount)
	if err != nil {
		return resp, err
	}

	// 创建订单及上游事务
	now := time.Now()
	order, tx, err := s.createOrderAndTransaction(merchant, req, products[0], amount, oid, now, settle)
	if err != nil {
		return resp, err
	}

	// ================== 调用上游通道 ==================
	var payUrl string
	var lastErr error
	for _, product := range products {
		payUrl, err = s.callUpstreamService(merchant, &req, &product, tx.UpOrderId)
		if err == nil {
			// 成功后清理
			s.clearUpstreamFail(
				uint64(product.UpstreamId),
				product.UpstreamCode,
				product.SysChannelCode,
			)

			// 异步更新订单绑定
			go func(p dto.PayProductVo) {
				if err := s.updateOrderBindOnSuccess(order, tx, merchant, p, amount, now); err != nil {
					log.Printf("[ORDER-BIND-UPDATE] ❌ 更新订单绑定失败: orderID=%d, upstream=%s, err=%v", order.OrderID, p.UpstreamCode, err)
					notify.Notify(system.BotChatID, "warn", "订单绑定更新失败",
						fmt.Sprintf("⚠️ OrderID: %d\n上游: %s\n错误: %v", order.OrderID, p.UpstreamCode, err), true)
				}
			}(product)

			// 异步更新通道成功率
			go func(pid int64) {
				if err := s.mainDao.UpdateSuccessRate(pid, true); err != nil {
					log.Printf("[SUCCESS-RATE] ❌ 更新通道成功率失败: productID=%d, err=%v", pid, err)
				}
			}(product.ID)
			break
		}
		// 当前上游失败
		s.recordUpstreamFail(
			uint64(product.UpstreamId),
			product.UpstreamTitle,
			product.UpstreamCode,
			product.SysChannelCode, // ✅ 系统通道编码
		)
		go func(pid int64) {
			if err := s.mainDao.UpdateSuccessRate(pid, false); err != nil {
				log.Printf("[SUCCESS-RATE] ❌ 更新通道成功率失败: productID=%d, err=%v", pid, err)
			}
		}(product.ID)
		lastErr = err
	}

	// 所有上游都失败
	if payUrl == "" && lastErr != nil {
		go func() {
			table := shard.OrderShard.GetTable(order.OrderID, now)
			dal.OrderDB.Table(table).Where("order_id = ?", order.OrderID).
				Updates(map[string]interface{}{"status": 5, "update_time": time.Now()})
		}()
		resp = dto.CreateOrderResp{
			TranFlow: req.TranFlow, PaySerialNo: strconv.FormatUint(oid, 10),
			Amount: req.Amount, Code: "001", SysTime: strconv.FormatInt(utils.GetTimestampMs(), 10),
		}
		return resp, lastErr
	}

	// 成功返回
	resp = dto.CreateOrderResp{
		TranFlow: req.TranFlow, PaySerialNo: strconv.FormatUint(oid, 10),
		Amount: req.Amount, Code: "0", Status: "0001",
		SysTime: strconv.FormatInt(utils.GetTimestampMs(), 10), Yul1: payUrl,
	}

	// 异步缓存订单 & 发布统计消息
	go s.asyncPostOrderCreation(oid, order, merchant.MerchantID, req.TranFlow, req.Amount, now)
	// 异步发布订单统计事件
	go s.publishOrderStat(order)

	return resp, nil
}

// ================== 平滑加权轮询 + 均匀分配 ==================
func (s *ReceiveOrderService) selectWeightedPollingChannel(
	merchantID uint, sysChannelCode string, channelType int8, currency string, amount decimal.Decimal,
) ([]dto.PayProductVo, error) {

	// 获取当前商户可用通道
	products, err := s.mainDao.GetAvailablePollingPayProducts(merchantID, sysChannelCode, currency, channelType)
	if err != nil || len(products) == 0 {
		return nil, errors.New("no channel products available")
	}

	// 动态降权（近5分钟失败≥3次则降半）
	for i := range products {
		failCnt := s.getUpstreamFailCount(
			uint64(products[i].UpstreamId),
			products[i].UpstreamCode,
			products[i].SysChannelCode,
		)
		if failCnt >= 3 {
			newWeight := int(math.Max(1, float64(products[i].UpstreamWeight/2)))
			log.Printf("[WEIGHT-DECAY] 上游=%d 失败次数=%d, 权重降为 %d", products[i].UpstreamId, failCnt, newWeight)
			products[i].UpstreamWeight = newWeight
		}
	}

	// 组装加权map
	weights := make(map[int64]int)
	for _, p := range products {
		weights[p.ID] = p.UpstreamWeight
	}

	// 平滑加权轮询（Redis全局状态保存）
	key := fmt.Sprintf("rr_state:%s:%s", sysChannelCode, currency)
	selectedID := utils.SmoothWeightedRR(key, weights)

	// 主通道优先 + 备用通道按权重降序
	var ordered []dto.PayProductVo
	for _, p := range products {
		if p.ID == selectedID {
			ordered = append(ordered, p)
			break
		}
	}
	// 加入剩余通道（按权重排序）
	sort.SliceStable(products, func(i, j int) bool {
		return products[i].UpstreamWeight > products[j].UpstreamWeight
	})
	for _, p := range products {
		if p.ID != selectedID {
			ordered = append(ordered, p)
		}
	}

	// 金额范围过滤
	var filtered []dto.PayProductVo
	for _, p := range ordered {
		rangeStr := fmt.Sprintf("%v-%v", p.MinAmount, p.MaxAmount)
		if utils.MatchOrderRange(amount, rangeStr) {
			filtered = append(filtered, p)
		}
	}

	if len(filtered) == 0 {
		return nil, errors.New("no suitable channel found after weighted polling")
	}

	log.Printf("[CHANNEL-RR] currency=%s, selectedID=%d, total=%d, filtered=%d",
		currency, selectedID, len(products), len(filtered))

	return filtered, nil
}

// publishOrderStat 异步发布订单统计事件
func (s *ReceiveOrderService) publishOrderStat(ord *ordermodel.MerchantOrder) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[order_stat panic recovered] %v\n%s", r, debug.Stack())
			}
		}()

		// 查询国家信息
		country, cErr := s.mainDao.GetCountry(ord.Currency)
		if cErr != nil {
			notify.Notify(system.BotChatID, "warn", "代收下单统计",
				fmt.Sprintf("⚠️ order %v, 获取国家信息异常: err=%v currency=%v", ord.OrderID, cErr, ord.Currency), true)
			return
		}

		if s.pub == nil {
			log.Printf("[order_stat] publisher is nil, skip publish. order=%v", ord.OrderID)
			return
		}

		// 组装 MQ 消息体
		msg := &dto.OrderMessageMQ{
			OrderID:       strconv.FormatUint(ord.OrderID, 10),
			MerchantID:    ord.MID,
			CountryID:     country.ID,
			ChannelID:     ord.ChannelID,
			SupplierID:    ord.SupplierID,
			Amount:        ord.Amount,
			SuccessAmount: decimal.Zero,
			Profit:        decimal.Zero,
			Cost:          decimal.Zero,
			Fee:           decimal.Zero,
			Status:        1,
			OrderType:     "collect",
			Currency:      ord.Currency,
			CreateTime:    time.Now(),
		}

		// 入列统计队列
		if err := s.pub.Publish("order_stat", msg); err != nil {
			notify.Notify(system.BotChatID, "warn", "代收下单统计",
				fmt.Sprintf("⚠️ order %v, 统计数据入列失败: %v", ord.OrderID, err), true)
			return
		}

		log.Printf("[order_stat] 入列成功, order=%v, merchant=%v, channel=%v", ord.OrderID, ord.MID, ord.ChannelID)
	}()
}

// updateOrderBindOnSuccess 成功后将订单与实际成功的通道产品进行绑定，并重新计算费用/利润/快照
func (s *ReceiveOrderService) updateOrderBindOnSuccess(
	order *ordermodel.MerchantOrder,
	upTx *ordermodel.UpstreamTx,
	merchant *mainmodel.Merchant,
	product dto.PayProductVo,
	amount decimal.Decimal,
	now time.Time,
) error {
	// 重新计算结算（包含代理信息、商户费率、成本费率等）
	settle, err := s.calculateSettlement(merchant, product, amount)
	if err != nil {
		return fmt.Errorf("recalculate settlement failed: %w", err)
	}

	// 成本与利润重算（与 createOrder 一致的口径）
	costFee := amount.Mul(product.CostRate).Div(decimal.NewFromInt(100)).Add(product.CostFee)
	orderFee := amount.Mul(product.MDefaultRate).Div(decimal.NewFromInt(100)).Add(product.MSingleFee)
	profitFee := orderFee.Sub(costFee)

	// 拷贝结算快照结构（防止修改引用）
	var orderSettle dto.SettlementResult
	if err := copier.Copy(&orderSettle, &settle); err != nil {
		return fmt.Errorf("copy settlement snapshot failed: %w", err)
	}

	// ===== 更新订单表（绑定通道 + 费率 + 成本 + 利润 + 结算快照）=====
	orderTable := shard.OrderShard.GetTable(order.OrderID, now)
	updateOrder := map[string]interface{}{
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
		"country":          product.Country,
		"cost":             costFee,
		"profit":           profitFee,
		"currency":         product.Currency,
		"settle_snapshot":  ordermodel.SettleSnapshot(orderSettle), // ✅ 更新结算快照
		"update_time":      now,
	}

	if err := dal.OrderDB.Table(orderTable).
		Where("order_id = ?", order.OrderID).
		Updates(updateOrder).Error; err != nil {
		return fmt.Errorf("update order binding failed: %w", err)
	}

	// ===== 更新上游交易表（供应商ID可能变化）=====
	if upTx != nil {
		txTable := shard.UpOrderShard.GetTable(upTx.UpOrderId, now)
		updateTx := map[string]interface{}{
			"supplier_id": product.UpstreamId,
			"currency":    product.Currency,
			"update_time": now,
		}
		if err := dal.OrderDB.Table(txTable).
			Where("order_id = ? AND up_order_id = ?", upTx.OrderID, upTx.UpOrderId).
			Updates(updateTx).Error; err != nil {
			return fmt.Errorf("update upstream tx failed: %w", err)
		}
	}

	return nil
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
	costFee := amount.Mul(payChannelProduct.CostRate).Div(decimal.NewFromInt(100)) //上游成本费用
	costFee = costFee.Add(payChannelProduct.CostFee)
	orderFee := amount.Mul(payChannelProduct.MDefaultRate).Div(decimal.NewFromInt(100)) //商户手续费
	orderFee = orderFee.Add(payChannelProduct.MSingleFee)
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

	var bankName, bankCode string
	if req.BankCode != "" {
		// 根据接平台银行编码查询平台银行信息
		platformBank, pbErr := s.mainDao.QueryPlatformBankInfo(req.BankCode, merchant.Currency)
		if pbErr != nil {
			return "", fmt.Errorf(fmt.Sprintf("receive platform Bank code does not exist,%s", req.BankCode))
		}
		// 根据接口ID+平台银行编码+国家货币查询对应上游银行编码+银行名称
		upstreamBank, ubErr := s.mainDao.QueryUpstreamBankInfo(payChannelProduct.InterfaceID, req.BankCode, payChannelProduct.Currency)
		if ubErr != nil {
			if payChannelProduct.InterfacePayVerifyBank > 0 {
				return "", fmt.Errorf(fmt.Sprintf("receive upstream Bank code does not exist,%s", req.BankCode))
			} else {
				bankCode = platformBank.Code
				bankName = platformBank.Name
			}
		} else {
			bankCode = upstreamBank.UpstreamBankCode
			bankName = upstreamBank.UpstreamBankName
		}
	}

	upstreamRequest := dto.UpstreamRequest{
		Currency:          payChannelProduct.Currency,
		Amount:            req.Amount,
		RedirectUrl:       req.RedirectUrl,
		ProductInfo:       req.ProductInfo,
		PayType:           req.PayType,
		AccNo:             req.AccNo,
		AccName:           req.AccName,
		PayPhone:          req.PayPhone,
		ProviderKey:       payChannelProduct.InterfaceCode,
		MchOrderId:        strconv.FormatUint(txId, 10),
		ApiKey:            payChannelProduct.UpApiKey,
		SubmitUrl:         payChannelProduct.PayApi,
		QueryUrl:          payChannelProduct.PayoutQueryApi,
		MchNo:             payChannelProduct.UpAccount,
		UpstreamCode:      payChannelProduct.UpstreamCode,
		UpstreamTitle:     payChannelProduct.UpstreamTitle,
		IdentityType:      req.IdentityType,
		IdentityNum:       req.IdentityNum,
		BankCode:          bankCode,
		BankName:          bankName,
		PayMethod:         req.PayMethod,
		PayEmail:          req.PayEmail,
		NotifyUrl:         req.NotifyUrl,
		Mode:              "receive",
		ClientIp:          req.ClientId,
		DownstreamOrderNo: req.TranFlow,
	}

	// 使用带超时的上下文
	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
	defer cancel()

	// 调用上游服务
	mOrderId, upOrderNo, payUrl, err := CallUpstreamReceiveService(ctx, upstreamRequest, req)
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
