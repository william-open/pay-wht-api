package dao

import (
	"errors"
	"fmt"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"log"
	"time"
	"wht-order-api/internal/dal"
	"wht-order-api/internal/dto"
	mainmodel "wht-order-api/internal/model/main"
)

type MainDao struct {
	DB *gorm.DB
}

// 工厂方法：默认使用 dal.MainDB
func NewMainDao() *MainDao {
	if dal.MainDB == nil {
		log.Panic("[FATAL] dal.MainDB is nil - database not initialized")
	}
	return &MainDao{DB: dal.MainDB}
}

// 支持传入自定义 DB（比如 txDB）
func NewMainDaoWithDB(db *gorm.DB) *MainDao {
	if db == nil {
		log.Panic("[FATAL] db cannot be nil")
	}
	return &MainDao{DB: db}
}

// 安全检查方法
func (d *MainDao) checkDB() error {
	if d == nil {
		return errors.New("MainDao is nil")
	}
	if d.DB == nil {
		return errors.New("DB connection is nil")
	}
	return nil
}

func (d *MainDao) GetMerchant(mid string) (*mainmodel.Merchant, error) {
	if err := d.checkDB(); err != nil {
		return nil, fmt.Errorf("get merchant failed: %w", err)
	}

	var m mainmodel.Merchant
	if err := d.DB.Where("app_id=?", mid).First(&m).Error; err != nil {
		return nil, fmt.Errorf("query merchant failed: %w", err)
	}
	return &m, nil
}

func (d *MainDao) GetMerchantWhitelist(mid uint64, mode int8) ([]mainmodel.MerchantWhitelist, error) {
	if err := d.checkDB(); err != nil {
		return nil, fmt.Errorf("get merchant whitelist failed: %w", err)
	}

	var m []mainmodel.MerchantWhitelist
	query := d.DB
	if mode == 1 { //代收
		query = query.Where("can_receive = ?", 1)
	} else { //代付
		query = query.Where("can_payout = ?", 1)
	}
	if err := query.Where("m_id=?", mid).Find(&m).Error; err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	return m, nil
}

func (d *MainDao) GetMerchantId(mid string) (*mainmodel.Merchant, error) {
	if err := d.checkDB(); err != nil {
		return nil, fmt.Errorf("get merchant by id failed: %w", err)
	}

	var m mainmodel.Merchant
	if err := d.DB.Where("m_id=?", mid).First(&m).Error; err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	return &m, nil
}

// GetUpstreamWhitelist 上游供应商信息
func (d *MainDao) GetUpstreamWhitelist(upstreamId uint64) (*dto.VerifyUpstream, error) {
	if err := d.checkDB(); err != nil {
		return nil, fmt.Errorf("get upstream whitelist failed: %w", err)
	}

	var upstreamModel mainmodel.PayUpstream
	var upstream dto.VerifyUpstream
	if err := d.DB.Select("ip_white_list,status").Where("id=?", upstreamId).First(&upstreamModel).Error; err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if upstreamModel.IPWhiteList == nil {
		return nil, errors.New("ip white list is nil")
	}

	upstream.IpWhitelist = *upstreamModel.IPWhiteList
	upstream.Status = upstreamModel.Status
	return &upstream, nil
}

func (d *MainDao) GetChannel(cid uint64) (*mainmodel.Channel, error) {
	if err := d.checkDB(); err != nil {
		return nil, fmt.Errorf("get channel failed: %w", err)
	}

	var ch mainmodel.Channel
	if err := d.DB.Where("channel_id=?", cid).First(&ch).Error; err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	return &ch, nil
}

// 查询上游可用通道
func (d *MainDao) SelectPaymentChannel(merchantID uint, channelCode string, currency string) ([]dto.PaymentChannelVo, error) {
	if err := d.checkDB(); err != nil {
		return nil, fmt.Errorf("select payment channel failed: %w", err)
	}

	var resp []dto.PaymentChannelVo
	query := d.DB.Table("w_merchant_channel AS a").
		Select("c.title as up_channel_title,c.default_rate as up_channel_rate,c.add_rate as up_channel_fixed_fee,a.currency,a.sys_channel_id,a.up_channel_id,a.default_rate as m_default_rate,a.single_fee as m_single_fee,c.order_range,c.default_rate as up_default_rate,c.add_rate as up_single_fee,c.upstream_code,b.coding,b.title as sys_channel_title,c.weight,c.upstream_id,e.account AS up_account,e.receiving_key AS receiving_key,e.channel_code,f.country").
		Joins("JOIN w_pay_way AS b ON a.sys_channel_id = b.id").
		Joins("JOIN w_pay_upstream_product AS c ON a.up_channel_id = c.id").
		Joins("LEFT JOIN w_pay_upstream AS e ON c.upstream_id = e.id").
		Joins("LEFT JOIN w_currency_code AS f ON f.code = e.currency")
	query.Where("a.m_id =?", merchantID).
		Where("a.currency =?", currency).
		Where("a.status =?", 1).
		Where("b.status =?", 1).
		Where("c.status =?", 1).
		Where("b.coding =?", channelCode)
	query.Order("c.weight desc")
	if err := query.Find(&resp).Error; err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	return resp, nil
}

// GetAgentMerchant 查询代理商户
func (d *MainDao) GetAgentMerchant(param dto.QueryAgentMerchant) (*mainmodel.AgentMerchant, error) {
	if err := d.checkDB(); err != nil {
		return nil, fmt.Errorf("get agent merchant failed: %w", err)
	}

	var ch mainmodel.AgentMerchant
	if err := d.DB.Where("a_id=?", param.AId).Where("m_id=?", param.MId).Where("sys_channel_id=?", param.SysChannelID).First(&ch).Error; err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	return &ch, nil
}

// GetSysChannel 查询通道编码
func (d *MainDao) GetSysChannel(channelCode string) (*dto.PayWayVo, error) {
	if err := d.checkDB(); err != nil {
		return nil, fmt.Errorf("get sys channel failed: %w", err)
	}

	var ch dto.PayWayVo
	if err := d.DB.Table("w_pay_way").Where("coding=?", channelCode).Where("status=?", 1).First(&ch).Error; err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	return &ch, nil
}

// GetMerchantAccount 获取商户账户信息
func (d *MainDao) GetMerchantAccount(mId string) (dto.MerchantMoney, error) {
	if err := d.checkDB(); err != nil {
		return dto.MerchantMoney{}, fmt.Errorf("get merchant account failed: %w", err)
	}

	var ch dto.MerchantMoney
	if err := d.DB.Table("w_merchant_money").Where("uid=?", mId).First(&ch).Error; err != nil {
		return dto.MerchantMoney{}, fmt.Errorf("query failed: %w", err)
	}
	return ch, nil
}

// FreezeAdditionalAmount 增加商户冻结金额（补差额）
func (d *MainDao) FreezeAdditionalAmount(
	uid uint64,
	currency string,
	orderNo string,
	diff decimal.Decimal,
	operator string,
	mOrderNo string,
) error {
	if diff.LessThanOrEqual(decimal.Zero) {
		return nil // 不需要补充冻结
	}

	// 事务保护
	return dal.MainDB.Transaction(func(tx *gorm.DB) error {
		var oldBalance, newFreeze decimal.Decimal

		// 查询当前余额与冻结金额
		var row struct {
			Money       decimal.Decimal
			FreezeMoney decimal.Decimal
		}
		sqlSelect := "SELECT money, freeze_money FROM w_merchant_money WHERE uid = ? AND currency = ? FOR UPDATE"
		if err := tx.Raw(sqlSelect, uid, currency).Scan(&row).Error; err != nil {
			return fmt.Errorf("query merchant account failed: %w", err)
		}

		oldBalance = row.FreezeMoney
		newFreeze = oldBalance.Add(diff)

		// 更新冻结金额
		sqlUpdate := `
			UPDATE w_merchant_money 
			SET freeze_money = freeze_money + ?, update_time = NOW()
			WHERE uid = ? AND currency = ?`
		if err := tx.Exec(sqlUpdate, diff, uid, currency).Error; err != nil {
			return fmt.Errorf("update freeze_money failed: %w", err)
		}

		// 插入资金日志
		sqlLog := `
			INSERT INTO w_money_log 
			(uid, type, money, order_no, operator, currency, old_balance, balance, description, create_time, m_order_no)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
		if err := tx.Exec(sqlLog,
			uid,
			7, // type=7 => 后台冻结（冻结补差）
			diff,
			orderNo,
			operator,
			currency,
			oldBalance,
			newFreeze,
			"改派补充冻结",
			time.Now(),
			mOrderNo,
		).Error; err != nil {
			return fmt.Errorf("insert money log failed: %w", err)
		}

		return nil
	})
}

// QueryUpstreamBankInfo 接口ID+平台银行编码+货币符号查询上游银行信息
func (d *MainDao) QueryUpstreamBankInfo(interfaceId int, internalBankCode string, currency string) (dto.BankCodeMappingDto, error) {
	if err := d.checkDB(); err != nil {
		return dto.BankCodeMappingDto{}, fmt.Errorf("get merchant account failed: %w", err)
	}

	var ch dto.BankCodeMappingDto
	if err := d.DB.Table("w_bank_code_mapping").Where("interface_id=? and internal_bank_code=? and currency=? and status = ?", interfaceId, internalBankCode, currency, 1).First(&ch).Error; err != nil {
		return dto.BankCodeMappingDto{}, fmt.Errorf("query failed: %w", err)
	}
	return ch, nil
}

// QueryPlatformBankInfo 查询平台银行信息
func (d *MainDao) QueryPlatformBankInfo(internalBankCode string, currency string) (dto.BankCodeDto, error) {
	if err := d.checkDB(); err != nil {
		return dto.BankCodeDto{}, fmt.Errorf("get merchant account failed: %w", err)
	}

	var ch dto.BankCodeDto
	if err := d.DB.Table("w_bank_code").Where("currency = ? and code=? and  status=?", currency, internalBankCode, 1).First(&ch).Error; err != nil {
		return dto.BankCodeDto{}, fmt.Errorf("query failed: %w", err)
	}
	return ch, nil
}

// FreezePayout 创建代付订单时冻结资金并写冻结日志
func (d *MainDao) FreezePayout(uid uint64, currency, orderNo string, mOrderNo string, amount decimal.Decimal, operator string) error {
	if err := d.checkDB(); err != nil {
		return fmt.Errorf("freeze payout failed: %w", err)
	}
	if amount.LessThanOrEqual(decimal.Zero) {
		return fmt.Errorf("invalid freeze amount: %s", amount.String())
	}

	return d.DB.Transaction(func(tx *gorm.DB) error {
		// 获取并锁定商户账户（避免并发超额冻结）
		var account mainmodel.MerchantMoney
		if err := tx.Table("w_merchant_money").
			Where("uid = ? AND currency = ?", uid, currency).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&account).Error; err != nil {
			return fmt.Errorf("get merchant account failed: %w", err)
		}

		// 校验余额是否足够
		if account.Money.LessThan(amount) {
			return fmt.Errorf("insufficient balance: uid=%d, balance=%s, need=%s",
				uid, account.Money, amount)
		}

		oldBalance := account.Money
		oldFreeze := account.FreezeMoney
		newBalance := account.Money.Sub(amount)
		newFreeze := account.FreezeMoney.Add(amount)

		// 写冻结资金日志 (60)
		logEntry := mainmodel.MoneyLog{
			Currency:    currency,
			UID:         uid,
			Money:       amount.Neg(), // 扣减余额
			OrderNo:     orderNo,
			MOrderNo:    mOrderNo,
			Type:        dto.MoneyLogTypeFreeze,
			Description: fmt.Sprintf("代付下单冻结资金，冻结前=%s，冻结后=%s", oldFreeze, newFreeze),
			OldBalance:  oldBalance,
			Balance:     newBalance,
			Operator:    operator,
			CreateTime:  time.Now(),
			CreateBy:    operator,
		}

		if err := tx.Table("w_money_log").
			Clauses(clause.OnConflict{DoNothing: true}). // 幂等保护
			Create(&logEntry).Error; err != nil {
			return fmt.Errorf("create freeze log failed: %w", err)
		}

		// 如果日志没写成功，说明已处理过该订单，直接返回成功（幂等）
		if logEntry.ID == 0 {
			return nil
		}

		// 更新账户余额 + 冻结金额
		if err := tx.Table("w_merchant_money").
			Where("uid = ? AND currency = ?", uid, currency).
			Updates(map[string]interface{}{
				"money":        newBalance,
				"freeze_money": newFreeze,
				"update_time":  time.Now(),
			}).Error; err != nil {
			return fmt.Errorf("update merchant account failed: %w", err)
		}

		return nil
	})
}

// HandlePayoutCallback 处理代付回调资金逻辑
// status = true 表示代付成功；false 表示代付失败（解冻回余额）
func (d *MainDao) HandlePayoutCallback(uid uint64, currency, orderNo string, mOrderNo string, merchantFees decimal.Decimal, agentFees decimal.Decimal, status bool, orderAmount decimal.Decimal, operator string) error {
	if err := d.checkDB(); err != nil {
		return fmt.Errorf("handle payout callback failed: %w", err)
	}
	//if amount.LessThanOrEqual(decimal.Zero) {
	//	return fmt.Errorf("invalid amount: %s", amount.String())
	//}
	//订单原始费用+商户手续费+代理手续费
	orderAmount = orderAmount.Add(agentFees).Add(merchantFees)

	return d.DB.Transaction(func(tx *gorm.DB) error {
		// 获取商户账户
		var account mainmodel.MerchantMoney
		if err := tx.Table("w_merchant_money").
			Where("uid = ? AND currency = ?", uid, currency).
			First(&account).Error; err != nil {
			return fmt.Errorf("get merchant account failed: %w", err)
		}

		oldBalance := account.Money
		// oldFreeze := account.FreezeMoney // 未使用可删除

		// 冻结足额校验（成功/失败两种路径都需要先从冻结减掉）
		if account.FreezeMoney.LessThan(orderAmount) {
			return fmt.Errorf("insufficient frozen funds: uid=%d, frozen=%s, need=%s",
				uid, account.FreezeMoney.String(), orderAmount.String())
		}

		if status {
			// ===== 代付成功：仅从冻结扣减，不回余额 =====
			newFreeze := account.FreezeMoney.Sub(orderAmount)

			// 代付出账日志（余额不变，记录动作）
			payoutLog := mainmodel.MoneyLog{
				Currency:    currency,
				UID:         uid,
				Money:       orderAmount.Neg(), // 记账为出账金额；Balance 不变仅用于展示可用余额
				OrderNo:     orderNo,
				MOrderNo:    mOrderNo,
				Type:        dto.MoneyLogTypePayout,
				Description: "代付成功，扣除冻结资金",
				OldBalance:  oldBalance,
				Balance:     oldBalance,
				Operator:    operator,
				CreateTime:  time.Now(),
				CreateBy:    operator,
			}
			if err := tx.Table("w_money_log").
				Clauses(clause.OnConflict{DoNothing: true}).
				Create(&payoutLog).Error; err != nil {
				return fmt.Errorf("create payout log failed: %w", err)
			}

			// 更新冻结
			if err := tx.Table("w_merchant_money").
				Where("uid = ? AND currency = ?", uid, currency).
				Updates(map[string]interface{}{
					"freeze_money": newFreeze,
					"update_time":  time.Now(),
				}).Error; err != nil {
				return fmt.Errorf("update freeze money failed: %w", err)
			}
			return nil
		}

		// ===== 代付失败：冻结减掉 + 余额加回，写两条日志 =====
		newFreeze := account.FreezeMoney.Sub(orderAmount)
		newBalance := account.Money.Add(orderAmount)

		// 删除冻结日志 (62) —— 表示从冻结池扣除该笔冻结
		delFreezeLog := mainmodel.MoneyLog{
			Currency:    currency,
			UID:         uid,
			Money:       orderAmount.Neg(), // 动作金额（冻结侧减少）
			OrderNo:     orderNo,
			MOrderNo:    mOrderNo,
			Type:        dto.MoneyLogTypeUnfreezeDel,
			Description: "代付失败，取消冻结资金",
			OldBalance:  oldBalance,
			Balance:     oldBalance, // 可用余额此时仍未变
			CreateTime:  time.Now(),
			Operator:    operator,
			CreateBy:    operator,
		}
		if err := tx.Table("w_money_log").
			Clauses(clause.OnConflict{DoNothing: true}).
			Create(&delFreezeLog).Error; err != nil {
			return fmt.Errorf("create unfreezeDel log failed: %w", err)
		}

		// 解冻资金退回余额日志 (61)
		unfreezeLog := mainmodel.MoneyLog{
			Currency:    currency,
			UID:         uid,
			Money:       orderAmount, // 回到可用余额
			OrderNo:     orderNo,
			MOrderNo:    mOrderNo,
			Type:        dto.MoneyLogTypeUnfreeze,
			Description: "代付失败，解冻资金退回余额",
			OldBalance:  oldBalance,
			Balance:     newBalance,
			CreateTime:  time.Now(),
			Operator:    operator,
			CreateBy:    operator,
		}
		if err := tx.Table("w_money_log").
			Clauses(clause.OnConflict{DoNothing: true}).
			Create(&unfreezeLog).Error; err != nil {
			return fmt.Errorf("create unfreeze log failed: %w", err)
		}

		// 同步账户（余额 + 冻结）
		if err := tx.Table("w_merchant_money").
			Where("uid = ? AND currency = ?", uid, currency).
			Updates(map[string]interface{}{
				"money":        newBalance,
				"freeze_money": newFreeze,
				"update_time":  time.Now(),
			}).Error; err != nil {
			return fmt.Errorf("update merchant account failed: %w", err)
		}

		return nil
	})
}

// CreateMoneyLog 创建用户资金日志（带事务）
func (d *MainDao) CreateMoneyLog(moneyLog dto.MoneyLog) error {
	if err := d.checkDB(); err != nil {
		return fmt.Errorf("create money log failed: %w", err)
	}

	log.Printf("创建用户资金日志结算数据: %+v", moneyLog)

	return d.DB.Transaction(func(tx *gorm.DB) error {
		// 获取商户账户
		var account mainmodel.MerchantMoney
		errAcc := tx.Table("w_merchant_money").
			Where("uid = ? AND currency = ?", moneyLog.UID, moneyLog.Currency).
			First(&account).Error
		if errAcc != nil && !errors.Is(errAcc, gorm.ErrRecordNotFound) {
			return fmt.Errorf("get merchant money failed: %w", errAcc)
		}

		// 设置余额
		oldBalance := decimal.NewFromInt(0)
		if errAcc == nil {
			oldBalance = account.Money
		}

		// 插入资金日志（带唯一约束保护）
		newLog := mainmodel.MoneyLog{
			Currency:    moneyLog.Currency,
			UID:         moneyLog.UID,
			Money:       moneyLog.Money,
			OrderNo:     moneyLog.OrderNo,
			MOrderNo:    moneyLog.MOrderNo,
			Type:        moneyLog.Type,
			Operator:    moneyLog.Operator,
			Description: moneyLog.Description,
			OldBalance:  oldBalance,
			Balance:     oldBalance.Add(moneyLog.Money),
			CreateTime:  time.Now(),
			CreateBy:    moneyLog.CreateBy,
		}

		if err := tx.Table("w_money_log").
			Clauses(clause.OnConflict{DoNothing: true}).
			Create(&newLog).Error; err != nil {
			return fmt.Errorf("create money log failed: %w", err)
		}

		// 如果日志没插入（可能因为唯一约束冲突），直接返回成功，不更新账户
		if newLog.ID == 0 {
			return nil
		}

		// 更新或创建账户
		if errors.Is(errAcc, gorm.ErrRecordNotFound) {
			// 新建账户
			newAccount := mainmodel.MerchantMoney{
				UID:         moneyLog.UID,
				Currency:    moneyLog.Currency,
				Money:       moneyLog.Money,
				FreezeMoney: decimal.Zero,
				CreateTime:  time.Now(),
				UpdateTime:  time.Now(),
			}
			if err := tx.Table("w_merchant_money").Create(&newAccount).Error; err != nil {
				return fmt.Errorf("create merchant money failed: %w", err)
			}
		} else {
			// 更新账户余额
			if err := tx.Table("w_merchant_money").
				Where("uid = ? AND currency = ?", moneyLog.UID, moneyLog.Currency).
				Updates(map[string]interface{}{
					"money":       account.Money.Add(moneyLog.Money),
					"update_time": time.Now(),
				}).Error; err != nil {
				return fmt.Errorf("update merchant money failed: %w", err)
			}
		}

		return nil
	})
}

// UpsertMerchantMoney 更新或插入商户资金（带事务）
func (d *MainDao) UpsertMerchantMoney(merchantMoney dto.MerchantMoney) error {
	if err := d.checkDB(); err != nil {
		return fmt.Errorf("upsert merchant money failed: %w", err)
	}

	return d.DB.Transaction(func(tx *gorm.DB) error {
		var record mainmodel.MerchantMoney
		err := tx.Table("w_merchant_money").
			Where("uid = ? AND currency = ?", merchantMoney.UID, merchantMoney.Currency).
			First(&record).Error

		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 创建新记录
			newRecord := mainmodel.MerchantMoney{
				UID:         merchantMoney.UID,
				Currency:    merchantMoney.Currency,
				Money:       merchantMoney.Money,
				FreezeMoney: decimal.NewFromFloat(0),
				CreateTime:  time.Now(),
				UpdateTime:  time.Now(),
			}
			return tx.Table("w_merchant_money").Create(&newRecord).Error
		} else if err != nil {
			return fmt.Errorf("get merchant money failed: %w", err)
		}

		// 更新余额
		record.Money = record.Money.Add(merchantMoney.Money)
		record.UpdateTime = time.Now()
		return tx.Table("w_merchant_money").Save(&record).Error
	})
}

// CreateAgentMoneyLog 创建代理收益资金日志+更新账户余额+资金日志（带事务）
func (d *MainDao) CreateAgentMoneyLog(agentMoney dto.AgentMoney, moneyLogType int8, remark string) error {
	if err := d.checkDB(); err != nil {
		return fmt.Errorf("create agent money log failed: %w", err)
	}

	if agentMoney.AID <= 0 {
		return nil
	}
	if agentMoney.Money.LessThanOrEqual(decimal.Zero) {
		return nil
	}

	return d.DB.Transaction(func(tx *gorm.DB) error {
		// 检查是否已存在代理佣金记录（避免重复插入）
		var existing mainmodel.AgentMoney
		err := tx.Table("w_agent_money").
			Where("a_id = ? AND order_no = ? AND type = ?", agentMoney.AID, agentMoney.OrderNo, agentMoney.Type).
			First(&existing).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("check existing agent money failed: %w", err)
		}
		if err == nil {
			return nil // 已存在，直接返回
		}

		// 插入代理佣金日志
		agentLog := mainmodel.AgentMoney{
			Currency:   agentMoney.Currency,
			AID:        agentMoney.AID,
			MID:        agentMoney.MID,
			OrderNo:    agentMoney.OrderNo,
			MOrderNo:   agentMoney.MOrderNo,
			Type:       agentMoney.Type,
			OrderMoney: agentMoney.OrderMoney,
			Remark:     agentMoney.Remark,
			Money:      agentMoney.Money,
			CreateTime: time.Now(),
		}
		if err := tx.Table("w_agent_money").Create(&agentLog).Error; err != nil {
			return fmt.Errorf("create agent money log failed: %w", err)
		}

		// 查代理账户资金（统一使用 AID，而不是 MID）
		var account mainmodel.MerchantMoney
		errAcc := tx.Table("w_merchant_money").
			Where("uid = ? AND currency = ?", agentMoney.AID, agentMoney.Currency).
			First(&account).Error
		if errAcc != nil && !errors.Is(errAcc, gorm.ErrRecordNotFound) {
			return fmt.Errorf("get agent account failed: %w", errAcc)
		}

		// 生成资金日志
		oldBalance := decimal.Zero
		if errAcc == nil {
			oldBalance = account.Money
		}

		moneyLogModel := mainmodel.MoneyLog{
			Currency:    agentMoney.Currency,
			UID:         agentMoney.AID, // ✅ 改成代理 ID
			Money:       agentMoney.Money,
			OrderNo:     agentMoney.OrderNo,
			MOrderNo:    agentMoney.MOrderNo,
			Type:        moneyLogType, // 固定类型：代理代收佣金
			Operator:    "",           // 可传操作人
			Description: remark,
			OldBalance:  oldBalance,
			Balance:     oldBalance.Add(agentMoney.Money),
			CreateTime:  time.Now(),
		}

		if err := tx.Table("w_money_log").
			Clauses(clause.OnConflict{DoNothing: true}).
			Create(&moneyLogModel).Error; err != nil {
			return fmt.Errorf("create money log failed: %w", err)
		}

		// 如果日志没插入（可能因为唯一约束冲突），直接返回
		if moneyLogModel.ID == 0 {
			return nil
		}

		// 更新或创建代理资金账户
		if errors.Is(errAcc, gorm.ErrRecordNotFound) {
			newAccount := mainmodel.MerchantMoney{
				UID:         agentMoney.AID,
				Currency:    agentMoney.Currency,
				Money:       agentMoney.Money,
				FreezeMoney: decimal.Zero,
				CreateTime:  time.Now(),
				UpdateTime:  time.Now(),
			}
			if err := tx.Table("w_merchant_money").Create(&newAccount).Error; err != nil {
				return fmt.Errorf("create agent account failed: %w", err)
			}
		} else {
			updateData := map[string]interface{}{
				"money":       account.Money.Add(agentMoney.Money),
				"update_time": time.Now(),
			}
			if err := tx.Table("w_merchant_money").
				Where("uid = ? AND currency = ?", agentMoney.AID, agentMoney.Currency).
				Updates(updateData).Error; err != nil {
				return fmt.Errorf("update agent account failed: %w", err)
			}
		}

		return nil
	})
}

// WithTransaction 执行事务操作
func (d *MainDao) WithTransaction(fn func(tx *gorm.DB) error) error {
	if err := d.checkDB(); err != nil {
		return fmt.Errorf("with transaction failed: %w", err)
	}
	return d.DB.Transaction(fn)
}

// GetAccountDetail 获取商户指定货币账户信息
func (d *MainDao) GetAccountDetail(mId uint64, currency string) (dto.AccountResp, error) {
	if err := d.checkDB(); err != nil {
		return dto.AccountResp{}, fmt.Errorf("get account detail failed: %w", err)
	}

	var ch dto.AccountResp
	var chModel dto.Account
	if err := d.DB.Table("w_merchant_money AS a").
		Joins("inner join w_merchant AS b ON a.uid = b.m_id").
		Select("b.nickname as acc_name,b.app_id as merchant_no,a.money as amount,a.freeze_money as frozen_amount,a.currency").
		Where("a.uid=?", mId).
		Where("a.currency = ?", currency).
		Take(&chModel).Error; err != nil {
		return dto.AccountResp{}, fmt.Errorf("query failed: %w", err)
	}

	ch.Currency = chModel.Currency
	ch.MerchantNo = chModel.MerchantNo
	ch.Amount = chModel.Amount
	ch.FrozenAmount = chModel.FrozenAmount
	ch.AccName = chModel.AccName
	ch.Code = "0"
	ch.Msg = "成功"
	return ch, nil
}

// CheckChannelValid 查询商户通道是否启动
func (d *MainDao) CheckChannelValid(mid uint64, channelCode string) error {
	if err := d.checkDB(); err != nil {
		return fmt.Errorf("get account detail failed: %w", err)
	}
	var m mainmodel.MerchantChannel
	if err := dal.MainDB.Where("m_id=?", mid).Where("sys_channel_code = ?", channelCode).First(&m).Error; err != nil {
		return err
	}
	if m.Status < 1 {
		return errors.New("通道未开启")
	}
	if m.DefaultRate.Cmp(decimal.Zero) <= 0 {
		return errors.New("通道未开启")
	}
	return nil
}

// DetailChannel 查询商户通道详情
func (d *MainDao) DetailChannel(mid uint64, channelCode string) (*dto.MerchantChannelDTO, error) {
	if err := d.checkDB(); err != nil {
		return &dto.MerchantChannelDTO{}, fmt.Errorf("get merchant info detail failed: %w", err)
	}
	var m mainmodel.MerchantChannel
	var result *dto.MerchantChannelDTO

	// 查询商户通道
	if err := dal.MainDB.
		Where("m_id = ?", mid).
		Where("sys_channel_code = ?", channelCode).
		Where("status = ?", 1).
		First(&m).Error; err != nil {
		return result, err
	}

	// 校验通道状态
	if m.Status < 1 {
		return result, errors.New("通道未开启")
	}
	if m.DefaultRate.Cmp(decimal.Zero) <= 0 && m.Type == 1 {
		return result, errors.New("通道未设置有效费率")
	}

	// ✅ 转换为 DTO 返回
	result = &dto.MerchantChannelDTO{
		ID:             m.ID,
		MId:            m.MID,
		SysChannelCode: m.SysChannelCode,
		SysChannelId:   m.SysChannelID,
		Status:         m.Status,
		DefaultRate:    m.DefaultRate,
		SingleFee:      m.SingleFee,
		Currency:       m.Currency,
		DispatchMode:   m.DispatchMode,
		Type:           m.Type,
	}

	return result, nil
}

func (d *MainDao) UpdateSuccessRate(productID int64, success bool) error {
	if err := d.checkDB(); err != nil {
		return fmt.Errorf("update success rate  failed: %w", err)
	}
	// 可选：使用 Redis 或 DB 记录成功/失败次数
	// 简化处理：失败则衰减成功率
	if !success {
		return dal.MainDB.Exec(`
            UPDATE w_pay_product 
            SET success_rate = CASE 
                WHEN success_rate > 5 THEN success_rate * 0.95 
                ELSE success_rate 
            END 
            WHERE id = ?`, productID).Error
	}
	return nil
}

// GetAvailablePollingPayProducts 用于查询商户可用的支付通道产品，支持按通道类型、币种、状态筛选，并为轮询调度准备权重排序
func (d *MainDao) GetAvailablePollingPayProducts(mId uint, payType string, currency string, channelType int8) ([]dto.PayProductVo, error) {
	if err := d.checkDB(); err != nil {
		return []dto.PayProductVo{}, fmt.Errorf("GetAvailablePollingPayProducts: %w", err)
	}
	var products []dto.PayProductVo

	err := dal.MainDB.
		Table("w_pay_product AS p").
		Select(`
            p.id,p.title AS up_channel_title,
			p.currency,p.type,p.upstream_id,p.upstream_code,
			p.interface_id,p.sys_channel_id,p.sys_channel_code,p.status,
			p.cost_rate,p.cost_fee,p.min_amount,p.max_amount,p.fixed_amount,
			p.success_rate,
            u.weight AS upstream_weight,
			mc.default_rate AS m_default_rate,
			mc.single_fee AS m_single_fee,
			ui.code AS interface_code,
            ui.payout_verify_bank AS interface_payout_verify_bank,
            ui.pay_verify_bank AS interface_pay_verify_bank,
			wu.account AS up_account,
   			wu.md5_key AS up_api_key,
			wu.title AS upstream_title,
			wu.pay_api,
			wu.pay_query_api,
			wu.payout_api,
			wu.payout_query_api,
			wcc.country,
			wpw.title AS sys_channel_title
        `).
		Joins(`
            INNER JOIN w_merchant_channel_upstream AS u 
            ON p.id = u.up_channel_id
        `).
		Joins(`LEFT JOIN w_upstream_interface AS ui ON p.interface_id = ui.id`).
		Joins(`LEFT JOIN w_merchant_channel AS mc ON p.sys_channel_id = mc.sys_channel_id and mc.m_id = ?`, mId).
		Joins(`LEFT JOIN w_upstream AS wu ON p.upstream_id = wu.id`).
		Joins(`LEFT JOIN w_currency_code AS wcc ON p.currency = wcc.code`).
		Joins(`LEFT JOIN w_pay_way AS wpw ON p.sys_channel_id = wpw.id`).
		Where("p.status = ?", 1).
		Where("p.currency = ?", currency).
		Where("p.sys_channel_code = ?", payType).
		Where("p.type = ?", channelType).
		Where("u.m_id = ? AND u.currency = ? AND u.sys_channel_code = ? AND u.status = 1", mId, currency, payType).
		Order("u.weight DESC").
		Find(&products).Error

	return products, err
}

// GetTestSinglePayChannel 查询单独支付通道[管理后台测试通道]
func (d *MainDao) GetTestSinglePayChannel(
	mId uint,
	sysChannelCode string,
	channelType int8,
	currency string,
	payProductId uint64,
) (dto.PayProductVo, error) {
	if err := d.checkDB(); err != nil {
		return dto.PayProductVo{}, fmt.Errorf("GetSinglePayChannel failed : %w", err)
	}

	var product dto.PayProductVo

	query := dal.MainDB.
		Table("w_pay_product AS p").
		Select(`
            p.id, p.title AS up_channel_title,
            p.currency, p.type, p.upstream_id, p.upstream_code,
            p.interface_id, p.sys_channel_id, p.sys_channel_code, p.status,
            p.cost_rate, p.cost_fee, p.min_amount, p.max_amount, p.fixed_amount,
            p.success_rate,
            u.weight AS upstream_weight,
            mc.default_rate AS m_default_rate,
            mc.single_fee AS m_single_fee,
            ui.code AS interface_code,
            ui.payout_verify_bank AS interface_payout_verify_bank,
            ui.pay_verify_bank AS interface_pay_verify_bank,
            wu.account AS up_account,
            wu.title AS upstream_title,
            wu.md5_key AS up_api_key,
            wcc.country,
            wpw.title AS sys_channel_title
        `).
		// ⚡ 把 u.xxx 条件移到 JOIN 里面，并用 LEFT JOIN
		Joins(`
            LEFT JOIN w_merchant_channel_upstream AS u
            ON p.id = u.up_channel_id
            AND u.m_id = ?
            AND u.currency = ?
            AND u.sys_channel_code = ?
            AND u.status = 1
        `, mId, currency, sysChannelCode).
		Joins(`LEFT JOIN w_upstream_interface AS ui ON p.interface_id = ui.id`).
		Joins(`LEFT JOIN w_merchant_channel AS mc ON p.sys_channel_id = mc.sys_channel_id AND mc.m_id = ?`, mId).
		Joins(`LEFT JOIN w_upstream AS wu ON p.upstream_id = wu.id`).
		Joins(`LEFT JOIN w_currency_code AS wcc ON p.currency = wcc.code`).
		Joins(`LEFT JOIN w_pay_way AS wpw ON p.sys_channel_id = wpw.id`).
		Where("p.status = ?", 1).
		Where("p.currency = ?", currency).
		Where("p.sys_channel_code = ?", sysChannelCode).
		Where("p.type = ?", channelType).
		Where("p.id = ?", payProductId).
		Order("u.weight DESC")

	// 只取一条
	if err := query.Take(&product).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.PayProductVo{}, nil // 没找到，返回空对象
		}
		return dto.PayProductVo{}, err // 其他 SQL 错误
	}

	return product, nil
}

// GetSinglePayChannel 查询单独支付通道
func (d *MainDao) GetSinglePayChannel(
	mId uint,
	sysChannelCode string,
	channelType int8,
	currency string,
) (dto.PayProductVo, error) {
	if err := d.checkDB(); err != nil {
		return dto.PayProductVo{}, fmt.Errorf("GetSinglePayChannel failed : %w", err)
	}

	var product dto.PayProductVo

	query := dal.MainDB.
		Table("w_pay_product AS p").
		Select(`
            p.id, p.title AS up_channel_title,
            p.currency, p.type, p.upstream_id, p.upstream_code,
            p.interface_id, p.sys_channel_id, p.sys_channel_code, p.status,
            p.cost_rate, p.cost_fee, p.min_amount, p.max_amount, p.fixed_amount,
            p.success_rate,
            u.weight AS upstream_weight,
            mc.default_rate AS m_default_rate,
            mc.single_fee AS m_single_fee,
            ui.code AS interface_code,
            ui.payout_verify_bank AS interface_payout_verify_bank,
            ui.pay_verify_bank AS interface_pay_verify_bank,
            wu.account AS up_account,
            wu.title AS upstream_title,
            wu.md5_key AS up_api_key,
            wcc.country,
            wpw.title AS sys_channel_title
        `).
		// ⚡ 这里改成 LEFT JOIN，并把 u.xxx 条件挪到 ON 里
		Joins(`
            LEFT JOIN w_merchant_channel_upstream AS u
            ON p.id = u.up_channel_id
            AND u.m_id = ?
            AND u.currency = ?
            AND u.sys_channel_code = ?
            AND u.status = 1
        `, mId, currency, sysChannelCode).
		Joins(`LEFT JOIN w_upstream_interface AS ui ON p.interface_id = ui.id`).
		Joins(`LEFT JOIN w_merchant_channel AS mc ON p.sys_channel_id = mc.sys_channel_id AND mc.m_id = ?`, mId).
		Joins(`LEFT JOIN w_upstream AS wu ON p.upstream_id = wu.id`).
		Joins(`LEFT JOIN w_currency_code AS wcc ON p.currency = wcc.code`).
		Joins(`LEFT JOIN w_pay_way AS wpw ON p.sys_channel_id = wpw.id`).
		Where("p.status = ?", 1).
		Where("p.currency = ?", currency).
		Where("p.sys_channel_code = ?", sysChannelCode).
		Where("p.type = ?", channelType).
		Order("u.weight DESC")

	// 只取一条
	if err := query.Take(&product).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.PayProductVo{}, nil // 没找到，返回空对象
		}
		return dto.PayProductVo{}, err // SQL 执行错误
	}

	return product, nil
}

// GetCountry 获取国家信息
func (d *MainDao) GetCountry(currency string) (dto.CurrencyCodeResponse, error) {
	if err := d.checkDB(); err != nil {
		return dto.CurrencyCodeResponse{}, fmt.Errorf("get country  failed: %w", err)
	}

	var ch dto.CurrencyCodeResponse
	if err := d.DB.Table("w_currency_code").Where("code=?", currency).First(&ch).Error; err != nil {
		return dto.CurrencyCodeResponse{}, fmt.Errorf("query failed: %w", err)
	}
	return ch, nil
}

// GetUpstreamSupplier 获取上游供应商配置信息
func (d *MainDao) GetUpstreamSupplier(upstreamId uint64) (*dto.UpstreamSupplierDto, error) {
	if err := d.checkDB(); err != nil {
		return nil, fmt.Errorf("get upstream db connect failed: %w", err)
	}

	var upstream dto.UpstreamSupplierDto
	if err := d.DB.Table("w_upstream").Where("id=?", upstreamId).Scan(&upstream).Error; err != nil {
		return nil, fmt.Errorf("query upstream supplier failed: %w", err)
	}
	return &upstream, nil
}
