package auth

import "errors"

// 错误定义
var (
	ErrClientNotFound        = errors.New("未授权的客户端")
	ErrMissingAuthInfo       = errors.New("缺少必要的客户端验证信息")
	ErrInvalidSignature      = errors.New("无效的签名")
	ErrInvalidTimestamp      = errors.New("无效的时间戳")
	ErrExpiredTimestamp      = errors.New("时间戳已过期")
	ErrInvalidNonce          = errors.New("无效的随机数")
	ErrDuplicateRequest      = errors.New("重复的请求")
	ErrFailedToReadBody      = errors.New("读取请求体失败")
	ErrFailedToVerifyRequest = errors.New("请求验证失败")
)
