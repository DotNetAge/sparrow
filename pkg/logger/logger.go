package logger

import (
	"github.com/DotNetAge/sparrow/pkg/config"
	"go.uber.org/zap"
)

// Logger 日志包装器
type Logger struct {
	*zap.Logger
}

// New 创建新的日志实例
func NewLogger(cfg *config.LogConfig) (*Logger, error) {
	zapConfig := zap.NewProductionConfig()
	level, err := zap.ParseAtomicLevel(cfg.Level)
	if err != nil {
		level = zap.NewAtomicLevelAt(zap.InfoLevel) // 默认使用 Info 级别
	}
	zapConfig.Level = level

	if cfg.Format == "console" {
		zapConfig = zap.NewDevelopmentConfig()
		level, err = zap.ParseAtomicLevel(cfg.Level)
		if err != nil {
			level = zap.NewAtomicLevelAt(zap.InfoLevel) // 默认使用 Info 级别
		}
		zapConfig.Level = level
	}

	logger, err := zapConfig.Build()
	if err != nil {
		return nil, err
	}

	return &Logger{logger}, nil
}

// Info 记录信息日志
func (l *Logger) Info(msg string, fields ...interface{}) {
	l.Logger.Sugar().Infow(msg, fields...)
}

// Error 记录错误日志
func (l *Logger) Error(msg string, fields ...interface{}) {
	l.Logger.Sugar().Errorw(msg, fields...)
}

// Fatal 记录致命错误日志
func (l *Logger) Fatal(msg string, fields ...interface{}) {
	l.Logger.Sugar().Fatalw(msg, fields...)
}

// Debug 记录调试日志
func (l *Logger) Debug(msg string, fields ...interface{}) {
	l.Logger.Sugar().Debugw(msg, fields...)
}

// Warn 记录警告日志
func (l *Logger) Warn(msg string, fields ...interface{}) {
	l.Logger.Sugar().Warnw(msg, fields...)
}

// Infof 格式化信息日志
func (l *Logger) Infof(format string, args ...interface{}) {
	l.Logger.Sugar().Infof(format, args...)
}

// Errorf 格式化错误日志
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.Logger.Sugar().Errorf(format, args...)
}
