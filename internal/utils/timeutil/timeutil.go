package timeutil

import (
	"time"
)

// ===================== 基础函数 =====================

// NowUTC 返回当前 UTC 时间
func NowUTC() time.Time {
	return time.Now().UTC()
}

// NowShanghai 返回当前北京时间（Asia/Shanghai）
func NowShanghai() time.Time {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	return time.Now().In(loc)
}

// NowIn 返回指定时区的当前时间
func NowIn(tz string) (time.Time, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.Time{}, err
	}
	return time.Now().In(loc), nil
}

// ===================== 转换函数 =====================

// ToUTC 将任意时间转成 UTC
func ToUTC(t time.Time) time.Time {
	return t.UTC()
}

// ToLocal 将 UTC 时间转为指定时区
func ToLocal(t time.Time, tz string) (time.Time, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.Time{}, err
	}
	return t.In(loc), nil
}

// ===================== 格式化函数 =====================

// FormatISO8601 格式化为 ISO8601 / RFC3339 格式 (2025-10-03T06:45:21Z)
func FormatISO8601(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// FormatDate 格式化为 YYYY-MM-DD（常用于报表、stat_date）
func FormatDate(t time.Time, tz string) (string, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return "", err
	}
	return t.In(loc).Format("2006-01-02"), nil
}

// ===================== 解析函数 =====================

// ParseISO8601 解析 ISO8601 / RFC3339 时间字符串
func ParseISO8601(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

// ParseDate 解析日期 YYYY-MM-DD 到 UTC 时间
func ParseDate(s string, tz string) (time.Time, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.Time{}, err
	}
	return time.ParseInLocation("2006-01-02", s, loc)
}
