package dao

import (
	"errors"
	"fmt"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
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
		return nil, fmt.Errorf("query failed: %w", err)
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

// CreateMoneyLog 创建用户资金日志（带事务）
func (d *MainDao) CreateMoneyLog(moneyLog dto.MoneyLog) error {
	if err := d.checkDB(); err != nil {
		return fmt.Errorf("create money log failed: %w", err)
	}

	log.Printf("创建用户资金日志结算数据: %+v", moneyLog)
	return d.DB.Transaction(func(tx *gorm.DB) error {
		// 检查是否已存在记录
		var existing mainmodel.MoneyLog
		err := tx.Table("w_money_log").
			Where("uid = ? AND order_no = ? AND type = ?", moneyLog.UID, moneyLog.OrderNo, moneyLog.Type).
			First(&existing).Error

		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("check existing record failed: %w", err)
		}
		if err == nil {
			// 已存在，不重复插入
			return nil
		}

		// 获取商户账户信息
		var account mainmodel.MerchantMoney
		err = tx.Table("w_merchant_money").
			Where("uid = ? AND currency = ?", moneyLog.UID, moneyLog.Currency).
			First(&account).Error

		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("get merchant money failed: %w", err)
		}

		// 创建资金日志
		newLog := mainmodel.MoneyLog{
			Currency:    moneyLog.Currency,
			UID:         moneyLog.UID,
			Money:       moneyLog.Money,
			OrderNo:     moneyLog.OrderNo,
			Type:        moneyLog.Type,
			Operator:    moneyLog.Operator,
			Description: moneyLog.Description,
			OldBalance:  account.Money,
			Balance:     account.Money.Add(moneyLog.Money),
			CreateTime:  time.Now(),
		}

		if err := tx.Table("w_money_log").Create(&newLog).Error; err != nil {
			return fmt.Errorf("create money log failed: %w", err)
		}

		// 更新商户资金
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 创建新记录
			newAccount := mainmodel.MerchantMoney{
				UID:         moneyLog.UID,
				Currency:    moneyLog.Currency,
				Money:       moneyLog.Money,
				FreezeMoney: decimal.NewFromFloat(0),
				CreateTime:  time.Now(),
				UpdateTime:  time.Now(),
			}
			if err := tx.Table("w_merchant_money").Create(&newAccount).Error; err != nil {
				return fmt.Errorf("create merchant money failed: %w", err)
			}
		} else {
			// 更新现有记录
			updateData := map[string]interface{}{
				"money":       account.Money.Add(moneyLog.Money),
				"update_time": time.Now(),
			}
			if err := tx.Table("w_merchant_money").
				Where("uid = ? AND currency = ?", moneyLog.UID, moneyLog.Currency).
				Updates(updateData).Error; err != nil {
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

// CreateAgentMoneyLog 创建代理收益资金日志（带事务）
func (d *MainDao) CreateAgentMoneyLog(agentMoney dto.AgentMoney) error {
	if err := d.checkDB(); err != nil {
		return fmt.Errorf("create agent money log failed: %w", err)
	}

	if agentMoney.AID <= 0 {
		return nil
	}

	return d.DB.Transaction(func(tx *gorm.DB) error {
		// 检查是否已存在记录
		var existing mainmodel.AgentMoney
		err := tx.Table("w_agent_money").
			Where("a_id = ? AND order_no = ? AND type = ?", agentMoney.AID, agentMoney.OrderNo, agentMoney.Type).
			First(&existing).Error

		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("check existing agent money failed: %w", err)
		}
		if err == nil {
			// 已存在，不重复插入
			return nil
		}

		zero := decimal.NewFromFloat(0)
		if agentMoney.Money.LessThanOrEqual(zero) {
			return nil
		}

		// 创建代理资金日志
		newLog := mainmodel.AgentMoney{
			Currency:   agentMoney.Currency,
			AID:        agentMoney.AID,
			MID:        agentMoney.MID,
			OrderNo:    agentMoney.OrderNo,
			Type:       agentMoney.Type,
			OrderMoney: agentMoney.OrderMoney,
			Remark:     agentMoney.Remark,
			Money:      agentMoney.Money,
			CreateTime: time.Now(),
		}

		if err := tx.Table("w_agent_money").Create(&newLog).Error; err != nil {
			return fmt.Errorf("create agent money failed: %w", err)
		}

		// 更新代理商户资金
		var agentAccount mainmodel.MerchantMoney
		err = tx.Table("w_merchant_money").
			Where("uid = ? AND currency = ?", agentMoney.AID, agentMoney.Currency).
			First(&agentAccount).Error

		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 创建新记录
			newAccount := mainmodel.MerchantMoney{
				UID:         agentMoney.AID,
				Currency:    agentMoney.Currency,
				Money:       agentMoney.Money,
				FreezeMoney: decimal.NewFromFloat(0),
				CreateTime:  time.Now(),
				UpdateTime:  time.Now(),
			}
			if err := tx.Table("w_merchant_money").Create(&newAccount).Error; err != nil {
				return fmt.Errorf("create agent account failed: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("get agent account failed: %w", err)
		} else {
			// 更新现有记录
			updateData := map[string]interface{}{
				"money":       agentAccount.Money.Add(agentMoney.Money),
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
	if m.DefaultRate.Cmp(decimal.Zero) <= 0 {
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
