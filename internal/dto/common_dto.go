package dto

import (
	"github.com/shopspring/decimal"
	"time"
)

// UpstreamResponse 统一上游返回结构体
type UpstreamResponse struct {
	Code        int    `json:"code"`        // 状态码：0=成功，非0=失败
	Msg         string `json:"msg"`         // 状态描述
	UpOrderNo   string `json:"up_order_no"` // 上游订单号
	PayUrl      string `json:"pay_url"`     // 支付链接（H5、跳转地址）
	QrCode      string `json:"qr_code"`     // 二维码内容（扫码支付时使用）
	ExpireTime  int64  `json:"expire_time"` // 订单过期时间（时间戳，单位秒）
	Extra       string `json:"extra"`       // 上游返回的附加字段（透传用）
	RawResponse string `json:"-"`           // 原始响应内容（方便排查问题，不参与 JSON 序列化）
}

// NotifyMerchantPayload 通知商户消息
type NotifyMerchantPayload struct {
	TranFlow    string          `json:"tranFlow"`
	PaySerialNo string          `json:"paySerialNo"`
	Status      string          `json:"status"`
	Msg         string          `json:"msg"`
	MerchantNo  string          `json:"merchantNo"`
	Sign        string          `json:"sign"`
	Amount      decimal.Decimal `json:"amount"`
}

// UpstreamRequest 请求上游供应商参数
type UpstreamRequest struct {
	MchNo        string `json:"mchNo" binding:"required"`          //商户号
	ApiKey       string `json:"apiKey" binding:"required"`         //商户密钥
	MchOrderId   string `json:"mchOrderId" binding:"required"`     //商家订单号
	Amount       string `json:"amount" binding:"required"`         //交易金额
	PayType      string `json:"payType" binding:"required"`        //通道编码/支付类型
	NotifyUrl    string `json:"notifyUrl" binding:"omitempty,url"` //回调地址
	RedirectUrl  string `json:"redirectUrl"`                       //成功跳转地址
	Currency     string `json:"currency" binding:"required,len=3"` //货币符号
	ProductInfo  string `json:"productInfo" binding:"required"`    //订单标题/内容
	ProviderKey  string `json:"providerKey" binding:"required"`    //上游供应商对接编码
	Model        string `json:"model" binding:"required"`          //模式:receive和payout
	AccNo        string `json:"accNo"`                             //付款人账号
	AccName      string `json:"accName"`                           //付款人姓名
	PayEmail     string `json:"payEmail"`                          //付款人邮箱
	PayPhone     string `json:"payPhone"`                          //付款人手机号
	BankCode     string `json:"bankCode"`                          //付款人银行编码
	BankName     string `json:"bankName"`                          //付款人银行名
	IdentityType string `json:"identityType"`                      //证件类型
	IdentityNum  string `json:"identityNum"`                       //证件号
	PayMethod    string `json:"payMethod"`                         //支付方式
	Mode         string `json:"mode"`                              //模式 receive payout
	UpstreamCode string `json:"upstreamCode"`                      //上游供应商通道编码
	ClientIp     string `json:"clientIp"`                          //客户端IP地址
	SubmitUrl    string `json:"submitUrl"`                         //下单URL
	QueryUrl     string `json:"queryUrl"`                          //查单RL
	AccountType  string `json:"accountType"`                       //账户类型
	CciNo        string `json:"cciNo"`                             //银行间账户

}

// PayWayVo 系统支付通道信息
type PayWayVo struct {
	Id       uint64 `json:"Id"`
	Title    string `json:"title"`
	Currency string `json:"currency"`
	Coding   string `json:"coding"`
	Type     int8   `json:"type"`
	Status   int8   `json:"status"`
}

type UpdateUpTxVo struct {
	UpOrderId  uint64    `json:"column:up_order_id;primaryKey"`
	UpOrderNo  string    `json:"column:up_order_no"`
	UpdateTime time.Time `json:"column:update_time"`
}

type UpdateOrderVo struct {
	OrderId    uint64    `json:"column:orderId;primaryKey"`
	UpOrderId  uint64    `json:"column:upOrderId"`
	UpdateTime time.Time `json:"column:updateTime"`
}

// QueryAgentMerchant 查询代理商户信息
type QueryAgentMerchant struct {
	SysChannelID int64  `json:"sysChannelId"` // 系统通道编码ID
	UpChannelID  int64  `json:"upChannelId"`  // 上游通道编码ID
	AId          int64  `json:"aId"`          // 代理ID
	MId          int64  `json:"mId"`          // 商户ID
	Currency     string `json:"currency"`     // 货币符号
}

// PaymentChannelVo 支付通道信息 值对象
type PaymentChannelVo struct {
	MDefaultRate      decimal.Decimal `json:"mDefaultRate"` // 商户通道费率
	MSingleFee        decimal.Decimal `json:"mSingleFee"`   // 商户通道固定费用
	OrderRange        string          `json:"orderRange"`
	UpDefaultRate     decimal.Decimal `json:"upDefaultRate"` //上游通道费率
	UpSingleFee       decimal.Decimal `json:"upSingleFee"`   //上游通道固定费用
	UpstreamCode      string          `json:"upstreamCode"`  //上游通道编码
	Coding            string          `json:"coding"`        // 系统通道编码
	Weight            int             `json:"weight"`
	SysChannelID      int64           `json:"sysChannelId"`      // 系统通道编码ID
	UpstreamId        int64           `json:"upstreamId"`        // 上游供应商ID
	UpChannelId       int64           `json:"upChannelId"`       // 上游通道产品ID
	Currency          string          `json:"currency"`          // 货币符号
	UpAccount         string          `json:"upAccount"`         // 上游商户号
	ReceivingKey      string          `json:"receivingKey"`      // 上游代收密钥
	ChannelCode       string          `json:"channelCode"`       // 上游通道对接编码
	UpChannelTitle    string          `json:"upChannelTitle"`    // 上游通道名称
	UpChannelCode     string          `json:"upChannelCode"`     // 上游通道编码
	UpChannelRate     decimal.Decimal `json:"upChannelRate"`     // 上游通道费率
	UpChannelFixedFee string          `json:"upChannelFixedFee"` // 上游通道固定费用
	SysChannelTitle   string          `json:"sysChannelTitle"`   // 系统通道名称
	Country           string          `json:"country"`           // 国家
}

// SinglePayChannelVo 单独支付通道信息
type SinglePayChannelVo struct {
	MDefaultRate       decimal.Decimal `json:"mDefaultRate"`       // 商户通道费率
	MSingleFee         decimal.Decimal `json:"mSingleFee"`         // 商户通道固定费用
	FixedAmount        string          `json:"fixedAmount"`        // 固定可用金额多个值使用英文逗号分隔
	MinAmount          decimal.Decimal `json:"minAmount"`          //上游通道风控最小金额
	MaxAmount          decimal.Decimal `json:"maxAmount"`          //上游通道风控最大金额
	UpCostRate         decimal.Decimal `json:"upCostRate"`         //上游通道成本费率
	UpCostFee          decimal.Decimal `json:"upCostFee"`          //上游通道成本固定费用
	SuccessRate        decimal.Decimal `json:"successRate"`        //上游通道成功率
	UpstreamId         int64           `json:"upstreamId"`         //上游供应商ID
	UpstreamCode       string          `json:"upstreamCode"`       //上游通道编码
	SysChannelID       int64           `json:"sysChannelId"`       // 系统通道编码ID
	SysChannelCode     string          `json:"sysChannelCode"`     // 系统通道编码
	Status             int8            `json:"status"`             // 状态1:开启0:关闭
	Type               int8            `json:"type"`               // 通道类型：1:代收2:代付
	UpChannelProductId int64           `json:"upChannelProductId"` // 上游通道产品ID 也就是w_pay_product表的ID
	UpAccount          string          `json:"upAccount"`          // 上游商户号
	UpApiKey           string          `json:"upApiKey"`           // 上游APIKEY密钥
	Currency           string          `json:"currency"`           // 货币符号
	Country            string          `json:"country"`            // 国家
}

type ChannelInfo struct {
	// 系统通道信息
	SysChannelID   int64  // 系统通道编码ID
	SysChannelCode string // 系统通道编码

	// 上游通道信息
	UpstreamId     int             // 上游供应商ID
	UpstreamCode   string          // 上游通道编码
	UpChannelId    int64           // 上游通道产品ID（w_pay_product.id）
	UpChannelTitle string          // 上游通道名称（展示用）
	UpChannelRate  decimal.Decimal // 上游通道费率
	UpSingleFee    decimal.Decimal // 上游通道单笔费用

	// 商户通道信息
	MDefaultRate decimal.Decimal // 商户默认费率
	MSingleFee   decimal.Decimal // 商户单笔费用

	// 通道调度信息
	InterfaceCode string // 接口对接编码（用于上游调用调度服务）
	DispatchMode  int    // 调度模式：1=轮询权重，2=固定通道
	Weight        int    // 权重值（轮询排序用）

	// 其他信息
	Currency    string  // 币种
	Country     string  // 国家（可选）
	UpAccount   string  // 上游账号（商户编号）
	UpApiKey    string  // 上游API Key
	ChannelCode *string // 通道编码（用于上游请求）
}

type PayProductVo struct {
	ID              int64
	Title           string
	Currency        string
	Type            int8
	UpstreamId      int64
	UpstreamCode    string
	UpstreamTitle   string
	UpChannelTitle  string
	InterfaceID     int
	SysChannelID    int64
	SysChannelCode  string
	SysChannelTitle string
	// 商户通道信息
	MDefaultRate              decimal.Decimal // 商户默认费率
	MSingleFee                decimal.Decimal // 商户单笔费用
	Status                    int
	InterfacePayoutVerifyBank int
	InterfacePayVerifyBank    int
	CostRate                  decimal.Decimal
	CostFee                   decimal.Decimal
	MinAmount                 *decimal.Decimal
	MaxAmount                 *decimal.Decimal
	FixedAmount               string
	SuccessRate               decimal.Decimal
	UpAccount                 string
	UpApiKey                  string
	UpdateBy                  string
	Country                   string
	// 新增字段：从商户通道表中联查的权重
	UpstreamWeight int    `gorm:"column:upstream_weight"`
	InterfaceCode  string `gorm:"column:interface_code"`
	PayApi         string
	PayQueryApi    string
	PayoutApi      string
	PayoutQueryApi string
}

// VerifyUpstream 验证上游供应商信息
type VerifyUpstream struct {
	IpWhitelist string `json:"ipWhitelist"`
	Status      int8   `json:"status"`
}
