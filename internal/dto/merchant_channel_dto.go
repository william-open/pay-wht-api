package dto

import (
	"github.com/shopspring/decimal"
)

type MerchantChannelDTO struct {
	ID             uint64          `json:"id"`             // 主键
	MId            uint64          `json:"mId"`            // 商户ID
	SysChannelId   uint64          `json:"sysChannelId"`   // 系统通道ID
	Status         int8            `json:"status"`         // 状态: 1:开启0:关闭
	Type           int8            `json:"type"`           // 通道类型1表示代收2表示代付
	DispatchMode   int8            `json:"dispatchMode"`   // 调度模式：1=轮询权重，2=固定通道
	Currency       string          `json:"currency"`       // 货币
	SysChannelCode string          `json:"sysChannelCode"` // 系统通道编码
	DefaultRate    decimal.Decimal `json:"defaultRate"`    // 商户费率
	SingleFee      decimal.Decimal `json:"singleFee"`      // 商户单笔费用
}
