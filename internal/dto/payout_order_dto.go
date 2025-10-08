package dto

import "time"

// 代付请求参数
type CreatePayoutOrderReq struct {
	Version      string `json:"version" binding:"required"`         //接口版本
	MerchantNo   string `json:"merchant_no" binding:"required"`     //商户号
	TranFlow     string `json:"tran_flow" binding:"required"`       //订单号
	TranDatetime string `json:"tran_datetime" binding:"required"`   //13位时间戳
	Amount       string `json:"amount" binding:"required"`          //订单金额
	PayType      string `json:"pay_type" binding:"required"`        //通道编码
	NotifyUrl    string `json:"notify_url" binding:"omitempty,url"` //回调地址
	AccNo        string `json:"acc_no" binding:"required"`          // 账号
	AccName      string `json:"acc_name" binding:"required"`        //姓名
	PayMethod    string `json:"pay_method" binding:"required"`      //支付方式
	BankCode     string `json:"bank_code" binding:"required"`       //银行编码
	BankName     string `json:"bank_name" binding:"required"`       //银行名
	BranchBank   string `json:"branch_bank"`                        //支行银行名
	PayEmail     string `json:"pay_email"`                          //邮箱
	PayPhone     string `json:"pay_phone"`                          //手机号
	IdentityType string `json:"identity_type"`                      //证件类型
	IdentityNum  string `json:"identity_num"`                       //证件号码
	PayProductId string `json:"pay_product_id"`                     //上游支付产品ID【管理后台测试上游通道用】
	Sign         string `json:"sign" binding:"required"`            //MD5 签名 32大写
}

// CreatePayoutOrderResp 创建代付订单返回数据
type CreatePayoutOrderResp struct {
	Code        string `json:"code"`          //响应码
	Msg         string `json:"msg"`           //响应说明
	Status      string `json:"status"`        //订单状态
	TranFlow    string `json:"tran_flow"`     //商户订单号
	PaySerialNo string `json:"pay_serial_no"` //平台流水号
	SysTime     string `json:"sys_time"`      //系统当前日期
	Amount      string `json:"amount"`        //订单金额
	TraceID     string `json:"trace_id"`
}

type PayoutOrderVO struct {
	OrderID       uint64    `json:"order_id"`
	MerchantID    uint64    `json:"merchant_id"`
	MerchantOrdNo string    `json:"merchant_ord_no"`
	Amount        string    `json:"amount"`
	Currency      string    `json:"currency"`
	PayMethod     string    `json:"pay_method"`
	Status        int8      `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

// QueryPayoutOrderReq 代付查询参数
type QueryPayoutOrderReq struct {
	Version      string `json:"version" binding:"required"`       //接口版本
	MerchantNo   string `json:"merchant_no" binding:"required"`   //商户号
	TranFlow     string `json:"tran_flow" binding:"required"`     //订单号
	TranDatetime string `json:"tran_datetime" binding:"required"` //13位时间戳 (北京时间毫秒，与北京时间相差超过5分钟可能会无法查询)
	Sign         string `json:"sign" binding:"required"`          //MD5 签名 32大写
}

// QueryPayoutOrderResp 代付订单查询返回数据
type QueryPayoutOrderResp struct {
	Code        string      `json:"code"`
	Msg         string      `json:"msg"`
	Status      string      `json:"status"`
	TranFlow    string      `json:"tran_flow"`      //商户订单号
	PaySerialNo string      `json:"pay_serial_no"`  //平台流水号
	Amount      string      `json:"amount"`         //支付金额
	Yul1        interface{} `json:"yul1,omitempty"` //支付地址
	Yul2        string      `json:"yul2"`           // 备用 印度专属 印度UTR编号
	Yul3        string      `json:"yul3"`
	Yul4        string      `json:"yul4"`
	Yul5        string      `json:"yul5"`
}
