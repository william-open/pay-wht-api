package ordermodel

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"github.com/shopspring/decimal"
	"time"
)

type SettleSnapshot struct {
	OrderAmount      decimal.Decimal `json:"orderAmount"`
	MerchantFee      decimal.Decimal `json:"merchantFee"`
	MerchantFixed    decimal.Decimal `json:"merchantFixed"`
	MerchantTotalFee decimal.Decimal `json:"merchantTotalFee"`
	AgentFee         decimal.Decimal `json:"agentFee"`
	AgentFixed       decimal.Decimal `json:"agentFixed"`
	AgentTotalFee    decimal.Decimal `json:"agentTotalFee"`
	UpFeePct         decimal.Decimal `json:"upFeePct"`
	UpFixed          decimal.Decimal `json:"upFixed"`
	UpTotalFee       decimal.Decimal `json:"upTotalFee"`
	Currency         string          `json:"currency"` // 货币符号

	MerchantRecv   decimal.Decimal `json:"merchantRecv"`
	UpstreamCost   decimal.Decimal `json:"upstreamCost"`
	AgentIncome    decimal.Decimal `json:"agentIncome"`
	PlatformProfit decimal.Decimal `json:"platformProfit"`
}

func (s *SettleSnapshot) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("SettleSnapshot scan failed: %v", value)
	}
	return json.Unmarshal(bytes, s)
}

func (s SettleSnapshot) Value() (driver.Value, error) {
	return json.Marshal(s)
}

// MerchantOrder represents tpl_order
type MerchantOrder struct {
	OrderID        uint64           `gorm:"column:order_id;primaryKey" json:"orderId"`                            // 全局唯一订单ID
	MID            uint64           `gorm:"column:m_id;not null;index:idx_merchant_time" json:"mId"`              // 商户ID
	SupplierID     int64            `gorm:"column:supplier_id;not null" json:"supplierId"`                        // 上游供应商ID
	MOrderID       string           `gorm:"column:m_order_id;type:varchar(50);not null" json:"mOrderId"`          // 商户订单号
	Amount         decimal.Decimal  `gorm:"column:amount;type:decimal(18,4);not null" json:"amount"`              // 订单金额
	Fees           decimal.Decimal  `gorm:"column:fees;type:decimal(10,2);not null" json:"fees"`                  // 手续费
	PayAmount      decimal.Decimal  `gorm:"column:pay_amount;type:decimal(18,4);not null" json:"payAmount"`       // 实际支付金额
	RealMoney      decimal.Decimal  `gorm:"column:real_money;type:decimal(18,4);not null" json:"realMoney"`       // 实际到账金额
	FreezeAmount   decimal.Decimal  `gorm:"column:freeze_amount;type:decimal(18,4);not null" json:"freezeAmount"` // 冻结金额
	Currency       string           `gorm:"column:currency;type:char(3);not null" json:"currency"`                // 货币代码
	NotifyURL      string           `gorm:"column:notify_url;type:varchar(50);not null" json:"notifyUrl"`         // 异步回调通知URL
	ReturnURL      string           `gorm:"column:return_url;type:varchar(50);not null" json:"returnUrl"`         // 同步回调URL
	MDomain        string           `gorm:"column:m_domain;type:varchar(30);not null" json:"mDomain"`             // 下单域名
	MIP            string           `gorm:"column:m_ip;type:varchar(32);not null" json:"mIp"`                     // 下单IP
	Title          string           `gorm:"column:title;type:varchar(50);not null" json:"title"`                  // 订单标题
	AccountNo      string           `gorm:"column:account_no;type:varchar(30);not null" json:"accountNo"`         // 付款人账号
	AccountName    string           `gorm:"column:account_name;type:varchar(30);not null" json:"accountName"`     // 付款人姓名
	PayEmail       string           `gorm:"column:pay_email;type:varchar(30);not null" json:"payEmail"`           // 付款人邮箱
	PayPhone       string           `gorm:"column:pay_phone;type:varchar(30);not null" json:"payPhone"`           // 付款人手机号码
	BankCode       string           `gorm:"column:bank_code;type:varchar(30);not null" json:"bankCode"`           // 付款人银行编码
	BankName       string           `gorm:"column:bank_name;type:varchar(30);not null" json:"bankName"`           // 付款人银行名
	Status         int8             `gorm:"column:status;type:tinyint(1);not null" json:"status"`                 // 0:待支付,1:成功,2:失败,3:退款
	UpOrderID      *uint64          `gorm:"column:up_order_id" json:"upOrderId"`                                  // 上游交易订单ID
	ChannelID      int64            `gorm:"column:channel_id;not null" json:"channelId"`                          // 系统支付渠道ID
	UpChannelID    int64            `gorm:"column:up_channel_id;not null" json:"upChannelId"`                     // 上游通道ID
	NotifyStatus   *int8            `gorm:"column:notify_status;type:tinyint(1)" json:"notifyStatus"`             // 回调通知状态
	NotifyTime     *time.Time       `gorm:"column:notify_time;default null" json:"notifyTime"`                    // 回调通知时间
	CreateTime     *time.Time       `gorm:"column:create_time;autoCreateTime" json:"createTime"`                  // 创建时间
	UpdateTime     *time.Time       `gorm:"column:update_time;autoUpdateTime" json:"updateTime"`                  // 更新时间
	FinishTime     *time.Time       `gorm:"column:finish_time" json:"finishTime"`                                 // 完成时间
	MTitle         *string          `gorm:"column:m_title;type:varchar(30)" json:"mTitle"`                        // 商户名称
	ChannelCode    *string          `gorm:"column:channel_code;type:varchar(30)" json:"channelCode"`              // 通道编码
	ChannelTitle   *string          `gorm:"column:channel_title;type:varchar(30)" json:"channelTitle"`            // 通道名称
	UpChannelCode  *string          `gorm:"column:up_channel_code;type:varchar(30)" json:"upChannelCode"`         // 上游通道编码
	UpChannelTitle *string          `gorm:"column:up_channel_title;type:varchar(30)" json:"upChannelTitle"`       // 上游通道标题
	MRate          *decimal.Decimal `gorm:"column:m_rate;type:decimal(10,2)" json:"mRate"`                        // 商户费率
	UpRate         *decimal.Decimal `gorm:"column:up_rate;type:decimal(10,2)" json:"upRate"`                      // 上游通道费率
	Country        *string          `gorm:"column:country;type:varchar(30)" json:"country"`                       // 国家
	UpFixedFee     *decimal.Decimal `gorm:"column:up_fixed_fee;type:decimal(10,2)" json:"upFixedRate"`            // 上游通道固定费用
	MFixedFee      *decimal.Decimal `gorm:"column:m_fixed_fee;type:decimal(10,2)" json:"mFixedFee"`               // 商户通道固定费用
	Cost           *decimal.Decimal `gorm:"column:cost;type:decimal(10,2)" json:"cost"`                           // 成本费用
	Profit         *decimal.Decimal `gorm:"column:profit;type:decimal(10,2)" json:"profit"`                       // 利润费用
	SettleSnapshot SettleSnapshot   `gorm:"column:settle_snapshot;type:json" json:"settleSnapshot"`               // 订单结算快照
}
