package errs

import (
	"errors"
)

// 会话相关的错误定义
var (
	// ErrSessionNotFound 会话未找到错误
	ErrSessionNotFound = errors.New("session not found")

	// ErrInvalidSession 无效会话错误
	ErrInvalidSession = errors.New("invalid session")
)
