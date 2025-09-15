package utils

import (
	"encoding/json"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"log"
	"reflect"
	"strings"
	"time"
)

const ShardCount = 4

// MatchOrderRange 判断金额是否符合 orderRange 规则
func MatchOrderRange(amount decimal.Decimal, orderRange string) bool {
	rules := strings.Split(orderRange, ",")
	for _, rule := range rules {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}
		if strings.Contains(rule, "-") {
			// 区间规则
			bounds := strings.Split(rule, "-")
			if len(bounds) != 2 {
				continue
			}
			min, err1 := decimal.NewFromString(bounds[0])
			max, err2 := decimal.NewFromString(bounds[1])
			if err1 != nil || err2 != nil {
				continue
			}
			if amount.Cmp(min) >= 0 && amount.Cmp(max) <= 0 {
				return true
			}
		} else {
			// 固定金额规则
			val, err := decimal.NewFromString(rule)
			if err != nil {
				continue
			}
			if amount.Cmp(val) == 0 {
				return true
			}
		}
	}
	return false
}

// MapToJSON map转出为json
func MapToJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// JSONToMap 将JSON字符串转换为map或结构体
func JSONToMap(jsonStr string, target interface{}) error {
	if jsonStr == "" {
		return fmt.Errorf("empty JSON string")
	}

	// 检查target是否为指针
	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Ptr || targetValue.IsNil() {
		return fmt.Errorf("target must be a non-nil pointer")
	}

	// 解码JSON
	if err := json.Unmarshal([]byte(jsonStr), target); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return nil
}

// 分片表名生成器：p_order_{YYYYMM}_p{orderID % 4} 和 p_out_order_{YYYYMM}_p{orderID % 4}
func GetShardOrderTable(base string, orderID uint64, t time.Time) string {
	month := t.Format("200601")
	shard := orderID % 4
	return fmt.Sprintf("%s_%s_p%d", base, month, shard)
}

// 分片表名生成器：p_order_index_{YYYYMM}
func GetOrderIndexTable(base string, t time.Time) string {
	month := t.Format("200601")
	return fmt.Sprintf("%s_%s", base, month)
}

// GetOrderLogTable 根据订单号和时间定位日志表名
func GetOrderLogTable(base string, orderID uint64, t time.Time) string {
	if t.IsZero() {
		log.Printf("[GetOrderLogTable] 时间为空，使用当前时间")
		t = time.Now()
	}

	month := t.Format("200601") // 正确格式：202509
	shard := orderID % ShardCount
	return fmt.Sprintf("%s_%s_p%d", base, month, shard)
}

// 转化订单状态
func ConvertOrderStatus(status int8) string {
	var statusStr string
	switch status {
	case 1: //处理中
		statusStr = "0001"
	case 2: //成功
		statusStr = "0000"
	case 3: //冲正退回
		statusStr = "0005"
	}

	return statusStr

}

func PtrTime(t time.Time) *time.Time {
	return &t
}

// GenerateTraceID 生成链路追踪字段,全链路唯一标识，用于串联请求、日志、MQ、回调等所有环节
func GenerateTraceID() string {
	return uuid.New().String()
}

// SafeLogPrintf 使用安全的日志函数
func SafeLogPrintf(format string, args ...interface{}) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("日志打印时发生panic: %v", r)
			log.Printf("原始日志格式: %s", format)
			// 可以在这里记录原始参数的类型信息
			for i, arg := range args {
				if arg == nil {
					log.Printf("参数[%d]为nil", i)
				} else {
					log.Printf("参数[%d]类型: %T", i, arg)
				}
			}
		}
	}()
	log.Printf(format, args...)
}

// MustStringToDecimal 安全转换，如果失败返回零值
func MustStringToDecimal(amountStr string) decimal.Decimal {
	result, err := decimal.NewFromString(strings.TrimSpace(amountStr))
	if err != nil {
		return decimal.Zero
	}
	return result
}

// CheckNilPointersRecursive 检查结构体中的所有导出指针字段是否为 nil（递归）
func CheckNilPointersRecursive(obj interface{}) error {
	return checkNilRecursive(reflect.ValueOf(obj), "")
}

func checkNilRecursive(val reflect.Value, path string) error {
	if !val.IsValid() {
		return fmt.Errorf("invalid value at '%s'", path)
	}

	// 如果是指针，先解引用
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return fmt.Errorf("field '%s' is nil", path)
		}
		val = val.Elem()
	}

	// 只处理结构体
	if val.Kind() != reflect.Struct {
		return nil
	}

	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// 跳过未导出字段
		if fieldType.PkgPath != "" {
			continue
		}

		fieldPath := fieldType.Name
		if path != "" {
			fieldPath = path + "." + fieldPath
		}

		switch field.Kind() {
		case reflect.Ptr:
			if field.IsNil() {
				return fmt.Errorf("field '%s' is nil", fieldPath)
			}
			// 如果是指向结构体的指针，递归检查
			if field.Elem().Kind() == reflect.Struct {
				if err := checkNilRecursive(field, fieldPath); err != nil {
					return err
				}
			}
		case reflect.Struct:
			// 递归嵌套结构体
			if err := checkNilRecursive(field, fieldPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// 错误信息映射函数（可扩展）
func ValidationMsg(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "字段不能为空"
	case "url":
		return "必须是合法的 URL 地址"
	case "email":
		return "必须是合法的邮箱格式"
	default:
		return "参数格式错误"
	}
}
