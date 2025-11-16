package dto

type UpstreamSupplierDto struct {
	ID             int     `json:"id"`             // 主键
	Title          string  `json:"title"`          // 名称
	Type           int8    `json:"type"`           // 1:代收2:代付3:收付一体
	Account        string  `json:"account"`        // 上游供应商开户商户账号
	PayKey         string  `json:"payKey"`         // 代付密钥
	ReceivingKey   string  `json:"receivingKey"`   // 代收密钥
	AppID          *string `json:"appId"`          // appid
	AppSecret      string  `json:"appSecret"`      // 安全密钥
	Status         int8    `json:"status"`         // 状态0:关闭;1:开启
	PayAPI         *string `json:"payApi"`         // 代收下单API
	PayQueryAPI    *string `json:"payQueryApi"`    // 代收查询地址
	PayoutAPI      *string `json:"payoutApi"`      // 代付下单API
	PayoutQueryAPI *string `json:"payoutQueryApi"` // 代付查询地址
	BalanceInquiry *string `json:"balanceInquiry"` // 余额查询地址
	SendingAddress *string `json:"sendingAddress"` // 下发地址
	Supplementary  *string `json:"supplementary"`  // 补单地址
	NeedQuery      int8    `json:"needQuery"`      // 确定时是否需要查询
	IPWhiteList    *string `json:"ipWhiteList"`    // 回调IP白名单
	CallbackDomain *string `json:"callbackDomain"` // 回调访问的域名
	Currency       *string `json:"currency"`       // 国家货币符号
	ChannelCode    *string `json:"channelCode"`    // 通道对接编码
	Md5Key         *string `json:"md5Key"`         // md5密钥
	RsaPrivateKey  *string `json:"rsaPrivateKey"`  // 上游商户RSA私钥
	RsaPublicKey   *string `json:"rsaPublicKey"`   // 上游商户RSA公钥
	UpRsaPublicKey *string `json:"upRsaPublicKey"` //上游平台RSA公钥
	AuthUrl        *string `json:"authUrl"`        // API请求鉴权URL
	AgencyNo       *string `json:"agencyNo"`       // 机构号
	PayoutKey      *string `json:"payoutKey"`      // 代付密钥

}
