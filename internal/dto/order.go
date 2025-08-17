package dto

import "time"

type CreateOrderReq struct {
	Version      string `json:"version" binding:"required"`         //接口版本
	MerchantNo   string `json:"merchant_no" binding:"required"`     //商户号
	TranFlow     string `json:"tran_flow" binding:"required"`       //订单号
	TranDatetime string `json:"tran_datetime" binding:"required"`   //13位时间戳
	Amount       string `json:"amount" binding:"required"`          //订单金额
	PayType      string `json:"pay_type" binding:"required"`        //通道编码
	NotifyUrl    string `json:"notify_url" binding:"omitempty,url"` //回调地址
	RedirectUrl  string `json:"redirect_url"`                       //成功跳转地址
	Currency     string `json:"currency" binding:"required,len=3"`  //货币符号
	ProductInfo  string `json:"product_info" binding:"required"`    //订单标题/内容
	AccNo        string `json:"acc_no"`                             // 付款人账号
	AccName      string `json:"acc_name"`                           //付款人姓名
	PayEmail     string `json:"pay_email"`                          //付款人邮箱
	PayPhone     string `json:"pay_phone"`                          //付款人手机号
	BankCode     string `json:"bank_code"`                          //付款人银行名
	BankName     string `json:"bank_name"`                          //付款人银行名
	Sign         string `json:"sign" binding:"sign"`                //MD5 签名 32大写
}

// CreateOrderResp 创建订单返回数据
type CreateOrderResp struct {
	Code        string      `json:"code"`
	Msg         string      `json:"msg"`
	Status      string      `json:"status"`
	TranFlow    string      `json:"tran_flow"`
	PaySerialNo string      `json:"pay_serial_no"`
	SysTime     string      `json:"sys_time"`
	Amount      string      `json:"amount"`
	Yul1        interface{} `json:"yul1,omitempty"`
	Yul2        string      `json:"yul2"`
	Yul3        string      `json:"yul3"`
	Yul4        string      `json:"yul4"`
	Yul5        string      `json:"yul5"`
}

type OrderVO struct {
	OrderID       uint64    `json:"order_id"`
	MerchantID    uint64    `json:"merchant_id"`
	MerchantOrdNo string    `json:"merchant_ord_no"`
	Amount        string    `json:"amount"`
	Currency      string    `json:"currency"`
	PayMethod     string    `json:"pay_method"`
	Status        int8      `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

// PaymentChannelVo 支付通道信息
type PaymentChannelVo struct {
	MDefaultRate  float64 `json:"MDefaultRate"`
	MSingleFee    float64 `json:"MSingleFee"`
	OrderRange    string  `json:"orderRange"`
	UpDefaultRate float64 `json:"upDefaultRate"`
	UpSingleFee   float64 `json:"upSingleFee"`
	Coding        string  `json:"coding"`
	UpstreamCode  string  `json:"upstreamCode"`
	Weight        int     `json:"weight"`
	SysChannelID  int64   `json:"sysChannelId"` // 系统通道编码ID
	UpChannelID   int64   `json:"upChannelId"`  // 上游通道编码ID
	UpstreamId    int64   `json:"upstreamId"`   // 上游ID
	Currency      string  `json:"currency"`     // 货币符号
}

// QueryAgentMerchant 查询代理商户信息
type QueryAgentMerchant struct {
	SysChannelID int64  `json:"sysChannelId"` // 系统通道编码ID
	UpChannelID  int64  `json:"upChannelId"`  // 上游通道编码ID
	AId          int64  `json:"aId"`          // 代理ID
	MId          int64  `json:"mId"`          // 商户ID
	Currency     string `json:"currency"`     // 货币符号
}

// SettlementResult 结算数据
type SettlementResult struct {
	OrderAmount      float64
	MerchantFee      float64
	MerchantFixed    float64
	MerchantTotalFee float64
	AgentFee         float64
	AgentFixed       float64
	AgentTotalFee    float64
	UpFeePct         float64
	UpFixed          float64
	UpTotalFee       float64

	// outputs
	MerchantRecv   float64 // 商户最终到账（取决于模式）
	UpstreamCost   float64 // 上游成本/上游实际保留
	AgentIncome    float64 // 代理收入
	PlatformProfit float64 // 平台净利润
}

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
