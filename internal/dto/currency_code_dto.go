package dto

type CurrencyCodeResponse struct {
	ID      int64  `json:"id"`
	Code    string `json:"code"`
	NameEn  string `json:"nameEn"`
	NameZh  string `json:"nameZh"`
	Symbol  string `json:"symbol"`
	Country string `json:"country"`
	IsOpen  uint8  `json:"isOpen"`
}
