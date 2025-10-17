package codes

import (
	"fmt"
	"time"
)

// GenCode 生成前缀+时间戳+随机码式的编码
// prefix: 编码前缀（如 "ORD"）
// [业务标识（2-4位）][时间戳（精简版）][随机数/序列号]
func NewCode(prefix string) string {
	// 生成随机数（这里简单用时间戳的后 4 位）
	random := fmt.Sprintf("%04d", time.Now().UnixMilli()%10000)
	return fmt.Sprintf("%s%s%s", prefix, time.Now().Format("060102"), random)
}
