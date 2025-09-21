package dto

import "time"

// 代收请求参数
type CreateOrderReq struct {
	Version      string `json:"version" binding:"required"`         //接口版本
	MerchantNo   string `json:"merchant_no" binding:"required"`     //商户号
	TranFlow     string `json:"tran_flow" binding:"required"`       //订单号
	TranDatetime string `json:"tran_datetime" binding:"required"`   //13位时间戳
	Amount       string `json:"amount" binding:"required"`          //订单金额
	PayType      string `json:"pay_type" binding:"required"`        //通道编码
	NotifyUrl    string `json:"notify_url" binding:"omitempty,url"` //回调地址
	RedirectUrl  string `json:"redirect_url"`                       //成功跳转地址
	ProductInfo  string `json:"product_info" binding:"required"`    //订单标题/内容
	AccNo        string `json:"acc_no"`                             // 付款人账号
	AccName      string `json:"acc_name"`                           //付款人姓名
	PayEmail     string `json:"pay_email"`                          //付款人邮箱
	PayPhone     string `json:"pay_phone"`                          //付款人手机号
	BankCode     string `json:"bank_code"`                          //付款人银行名
	BankName     string `json:"bank_name"`                          //付款人银行名
	ClientId     string `json:"client_id"`                          //客户端IP
	Sign         string `json:"sign" binding:"required"`            //MD5 签名 32大写
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
	TraceID     string      `json:"trace_id"`
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

// QueryReceiveOrderReq 代收查询参数
type QueryReceiveOrderReq struct {
	Version      string `json:"version" binding:"required"`       //接口版本
	MerchantNo   string `json:"merchant_no" binding:"required"`   //商户号
	TranFlow     string `json:"tran_flow" binding:"required"`     //订单号
	TranDatetime string `json:"tran_datetime" binding:"required"` //13位时间戳
	Sign         string `json:"sign" binding:"required"`          //MD5 签名 32大写
}

// QueryReceiveOrderResp 代收订单查询返回数据
type QueryReceiveOrderResp struct {
	Code        string `json:"code"`
	Msg         string `json:"msg"`
	Status      string `json:"status"`
	TranFlow    string `json:"tran_flow"`     //商户订单号
	PaySerialNo string `json:"pay_serial_no"` //平台流水号
	Amount      string `json:"amount"`        //支付金额
}
