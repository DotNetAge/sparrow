package errs

import "errors"

// 预定义的领域错误
var (
	// ErrSnapshotNotFound 快照未找到错误
	ErrSnapshotNotFound = errors.New("snapshot not found")
)