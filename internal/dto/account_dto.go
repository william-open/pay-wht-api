package dto

// AccountReq 账户请求参数
type AccountReq struct {
	Version      string `json:"version" binding:"required"`       //接口版本
	MerchantNo   string `json:"merchant_no" binding:"required"`   //商户号
	Currency     string `json:"currency"`                         // 货币符号
	TranDatetime string `json:"tran_datetime" binding:"required"` //13位时间戳
	Sign         string `json:"sign" binding:"required"`          //MD5 签名 32大写
}

type Account struct {
	AccName      string `json:"acc_name"`      //  商户名称
	MerchantNo   string `json:"merchant_no"`   // 商户号
	Currency     string `json:"currency"`      // 货币符号
	FrozenAmount string `json:"frozen_amount"` //冻结金额
	Amount       string `json:"amount"`        // 可用余额
}

// AccountResp 账户返回数据
type AccountResp struct {
	Code         string `json:"code"`
	Msg          string `json:"msg"`
	AccName      string `json:"acc_name"`      //  商户名称
	MerchantNo   string `json:"merchant_no"`   // 商户号
	Currency     string `json:"currency"`      // 货币符号
	FrozenAmount string `json:"frozen_amount"` //冻结金额
	Amount       string `json:"amount"`        // 可用余额
}
