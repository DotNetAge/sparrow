package utils

import (
	"fmt"
	"strconv"
	"time"
)

// ToString 将interface{}转换为string
func ToString(v interface{}) string {
	if v == nil {
		return ""
	}
	if str, ok := v.(string); ok {
		return str
	}
	return fmt.Sprintf("%v", v)
}

// ParseTime 解析时间
func ParseTime(v interface{}) time.Time {
	if v == nil {
		return time.Time{}
	}

	// 尝试不同类型的时间格式
	switch t := v.(type) {
	case string:
		parsed, err := time.Parse(time.RFC3339, t)
		if err == nil {
			return parsed
		}
		// 尝试其他常见格式
		parsed, err = time.Parse("2006-01-02 15:04:05", t)
		if err == nil {
			return parsed
		}
	case time.Time:
		return t
	case float64:
		// 处理时间戳格式
		return time.Unix(int64(t), 0)
	}

	return time.Time{}
}

// ToInt 将interface{}转换为int
func ToInt(v interface{}) int {
	if v == nil {
		return 0
	}

	switch num := v.(type) {
	case int:
		return num
	case int64:
		return int(num)
	case float64:
		return int(num)
	case string:
		if val, err := strconv.Atoi(num); err == nil {
			return val
		}
	}

	return 0
}
