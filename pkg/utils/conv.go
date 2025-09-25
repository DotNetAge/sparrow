package utils

import (
	"fmt"
	"strconv"
	"strings"
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

// Camel 将下划线命名转换为驼峰命名（首字母小写）
// 例如：user_name -> userName
func Camel(s string) string {
	parts := strings.Split(s, "_")
	for i := range parts {
		if i > 0 {
			parts[i] = strings.Title(parts[i])
		}
	}
	return strings.Join(parts, "")
}

// Pascal 将下划线命名转换为帕斯卡命名（首字母大写）
// 例如：user_name -> UserName
func Pascal(s string) string {
	parts := strings.Split(s, "_")
	for i := range parts {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

// Snake 将驼峰或帕斯卡命名转换为下划线命名
// 例如：UserName -> user_name, userName -> user_name
func Snake(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

// Plural 将单词转换为复数形式（简单规则）
// 例如：user -> users, category -> categories
func Plural(s string) string {
	if len(s) == 0 {
		return s
	}

	// 处理特殊情况
	switch s {
	case "quiz":
		return "quizzes"
	}

	last := s[len(s)-1]
	switch last {
	case 's', 'x', 'z':
		return s + "es"
	case 'y':
		if len(s) > 1 {
			prev := s[len(s)-2]
			if prev != 'a' && prev != 'e' && prev != 'i' && prev != 'o' && prev != 'u' {
				return s[:len(s)-1] + "ies"
			}
		}
		return s + "s"
	default:
		return s + "s"
	}
}

// Kebab 将字符串转换为kebab-case格式（短横线命名）
// 例如：UserName -> user-name, userName -> user-name, user_name -> user-name
func Kebab(s string) string {
	if s == "" {
		return ""
	}

	// 先处理下划线为短横线
	s = strings.ReplaceAll(s, "_", "-")

	var result strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			// 大写字母转换为小写
			lower := r + 32

			// 在以下情况添加短横线：
			// 1. 不是第一个字符
			// 2. 前一个字符是小写字母或数字
			// 3. 当前是连续大写字母中的最后一个（下一个字符是小写）
			if i > 0 {
				prev := s[i-1]
				shouldAddDash := false

				// 如果前一个字符是小写或数字，添加短横线
				if (prev >= 'a' && prev <= 'z') || (prev >= '0' && prev <= '9') {
					shouldAddDash = true
				}

				// 如果是连续大写字母中的最后一个
				if i < len(s)-1 {
					next := s[i+1]
					if prev >= 'A' && prev <= 'Z' && (next >= 'a' && next <= 'z') {
						shouldAddDash = true
					}
				}

				if shouldAddDash {
					result.WriteRune('-')
				}
			}

			result.WriteRune(lower)
		} else {
			result.WriteRune(r)
		}
	}

	// 清理连续的短横线和首尾短横线
	resultStr := result.String()
	resultStr = strings.ReplaceAll(resultStr, "--", "-")
	return strings.Trim(resultStr, "-")
}
