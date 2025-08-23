package dto

import "time"

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
	TranFlow    string  `json:"tranFlow"`
	PaySerialNo string  `json:"paySerialNo"`
	Status      string  `json:"status"`
	Msg         string  `json:"msg"`
	MerchantNo  string  `json:"merchantNo"`
	Sign        string  `json:"sign"`
	Amount      float64 `json:"amount"`
}

// 请求上游供应商参数 UpstreamRequest
type UpstreamRequest struct {
	MchNo       string `json:"mchNo" binding:"required"`          //商户号
	ApiKey      string `json:"apiKey" binding:"required"`         //商户密钥
	MchOrderId  string `json:"mchOrderId" binding:"required"`     //商家订单号
	Amount      string `json:"amount" binding:"required"`         //交易金额
	PayType     string `json:"payType" binding:"required"`        //通道编码/支付类型
	NotifyUrl   string `json:"notifyUrl" binding:"omitempty,url"` //回调地址
	RedirectUrl string `json:"redirectUrl"`                       //成功跳转地址
	Currency    string `json:"currency" binding:"required,len=3"` //货币符号
	ProductInfo string `json:"productInfo" binding:"required"`    //订单标题/内容
	ProviderKey string `json:"providerKey" binding:"required"`    //上游供应商对接编码
	AccNo       string `json:"accNo"`                             //付款人账号
	AccName     string `json:"accName"`                           //付款人姓名
	PayEmail    string `json:"payEmail"`                          //付款人邮箱
	PayPhone    string `json:"payPhone"`                          //付款人手机号
	BankCode    string `json:"bankCode"`                          //付款人银行编码
	BankName    string `json:"bankName"`                          //付款人银行名
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

// PaymentChannelVo 支付通道信息
type PaymentChannelVo struct {
	MDefaultRate      float64 `json:"mDefaultRate"` // 商户通道费率
	MSingleFee        float64 `json:"mSingleFee"`   // 商户通道固定费用
	OrderRange        string  `json:"orderRange"`
	UpDefaultRate     float64 `json:"upDefaultRate"` //上游通道费率
	UpSingleFee       float64 `json:"upSingleFee"`   //上游通道固定费用
	UpstreamCode      string  `json:"upstreamCode"`  //上游通道编码
	Coding            string  `json:"coding"`        // 系统通道编码
	Weight            int     `json:"weight"`
	SysChannelID      int64   `json:"sysChannelId"`      // 系统通道编码ID
	UpstreamId        int64   `json:"upstreamId"`        // 上游供应商ID
	UpChannelId       int64   `json:"upChannelId"`       // 上游通道产品ID
	Currency          string  `json:"currency"`          // 货币符号
	UpAccount         string  `json:"upAccount"`         // 上游商户号
	ReceivingKey      string  `json:"receivingKey"`      // 上游代收密钥
	ChannelCode       string  `json:"channelCode"`       // 上游通道对接编码
	UpChannelTitle    string  `json:"upChannelTitle"`    // 上游通道名称
	UpChannelCode     string  `json:"upChannelCode"`     // 上游通道编码
	UpChannelRate     float64 `json:"upChannelRate"`     // 上游通道费率
	UpChannelFixedFee string  `json:"upChannelFixedFee"` // 上游通道固定费用
	SysChannelTitle   string  `json:"sysChannelTitle"`   // 系统通道名称
	Country           string  `json:"country"`           // 系统通道名称
}
