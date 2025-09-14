package mainmodel

import "github.com/shopspring/decimal"

// PayUpstream represents w_pay_upstream
type PayUpstream struct {
	ID             int             `gorm:"column:id;primaryKey;autoIncrement" json:"id"`                           // 主键
	Title          string          `gorm:"column:title;type:varchar(40);not null" json:"title"`                    // 名称
	Type           int8            `gorm:"column:type;type:tinyint(1);not null" json:"type"`                       // 1:代收2:代付3:收付一体
	WayID          string          `gorm:"column:way_id;type:json;not null" json:"wayId"`                          // 对应通道
	Account        string          `gorm:"column:account;type:varchar(100);not null" json:"account"`               // 上游供应商开户商户账号
	PayKey         string          `gorm:"column:pay_key;type:text;not null" json:"payKey"`                        // 代付密钥
	ReceivingKey   string          `gorm:"column:receiving_key;type:text;not null" json:"receivingKey"`            // 代收密钥
	SuccessRate    decimal.Decimal `gorm:"column:success_rate;type:double(5,2);default:100.00" json:"successRate"` // 成功率
	OrderQuantity  int             `gorm:"column:order_quantity;default:0" json:"orderQuantity"`                   // 总的订单数
	Rate           decimal.Decimal `gorm:"column:rate;type:double(4,2);not null" json:"rate"`                      // 默认费率
	AppID          *string         `gorm:"column:app_id;type:varchar(20)" json:"appId"`                            // appid
	AppSecret      string          `gorm:"column:app_secret;type:text;not null" json:"appSecret"`                  // 安全密钥
	UpdateTime     *int            `gorm:"column:update_time" json:"updateTime"`                                   // 上次更改时间
	ControlStatus  int8            `gorm:"column:control_status;type:tinyint(1);default:0" json:"controlStatus"`   // 风控状态:0否1是
	Sort           int             `gorm:"column:sort;default:0" json:"sort"`                                      // 排序
	PayingMoney    decimal.Decimal `gorm:"column:paying_money;type:decimal(16,2);default:0.00" json:"payingMoney"` // 当天交易金额
	MinMoney       decimal.Decimal `gorm:"column:min_money;type:decimal(15,2);default:0.00" json:"minMoney"`       // 单笔最小交易额
	MaxMoney       decimal.Decimal `gorm:"column:max_money;type:decimal(15,2);default:0.00" json:"maxMoney"`       // 单笔最大交易额
	Status         int8            `gorm:"column:status;type:tinyint(1);default:0" json:"status"`                  // 状态0:关闭;1:开启
	PayStatus      int8            `gorm:"column:pay_status;type:tinyint(1);default:1" json:"payStatus"`           // 状态0:关闭;1:开启;2:;3:系统错误
	OutStatus      int8            `gorm:"column:out_status;type:tinyint(1);default:1" json:"outStatus"`           // 状态0:关闭;1:开启;2:;3:系统错误
	PayAPI         *string         `gorm:"column:pay_api;type:varchar(100)" json:"payApi"`                         // 代收下单API
	PayQueryAPI    *string         `gorm:"column:pay_query_api;type:varchar(100)" json:"payQueryApi"`              // 代收查询地址
	PayoutAPI      *string         `gorm:"column:payout_api;type:varchar(100)" json:"payoutApi"`                   // 代付下单API
	PayoutQueryAPI *string         `gorm:"column:payout_query_api;type:varchar(100)" json:"payoutQueryApi"`        // 代付查询地址
	BalanceInquiry *string         `gorm:"column:balance_inquiry;type:varchar(100)" json:"balanceInquiry"`         // 余额查询地址
	SendingAddress *string         `gorm:"column:sending_address;type:varchar(100)" json:"sendingAddress"`         // 下发地址
	Supplementary  *string         `gorm:"column:supplementary;type:varchar(100)" json:"supplementary"`            // 补单地址
	Documentation  *string         `gorm:"column:documentation;type:varchar(100)" json:"documentation"`            // 文档地址
	NeedQuery      int8            `gorm:"column:need_query;type:tinyint(1);default:1" json:"needQuery"`           // 确定时是否需要查询
	IPWhiteList    *string         `gorm:"column:ip_white_list;type:varchar(200)" json:"ipWhiteList"`              // 回调IP白名单
	CallbackDomain *string         `gorm:"column:callback_domain;type:varchar(50)" json:"callbackDomain"`          // 回调访问的域名
	Remark         *string         `gorm:"column:remark;type:varchar(50)" json:"remark"`                           // 备注
	Currency       *string         `gorm:"column:currency;type:varchar(50)" json:"currency"`                       // 国家货币符号
	ChannelCode    *string         `gorm:"column:channel_code;type:varchar(255)" json:"channelCode"`               // 通道对接编码
	Md5Key         *string         `gorm:"column:md5_key;type:varchar(255)" json:"md5Key"`                         // md5密钥
	RsaPrivateKey  *string         `gorm:"column:rsa_private_key;type:varchar(500)" json:"rsaPrivateKey"`          // RSA私钥
	RsaPublicKey   *string         `gorm:"column:rsa_public_key;type:varchar(500)" json:"rsaPublicKey"`            // RSA公钥
}

func (PayUpstream) TableName() string { return "w_upstream" }
