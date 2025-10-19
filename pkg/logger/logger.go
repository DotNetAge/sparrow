package logger

import (
	"strings"

	"github.com/DotNetAge/sparrow/pkg/config"
	"go.uber.org/zap"
)

// Logger 日志包装器
type Logger struct {
	*zap.Logger
}

// New 创建新的日志实例
func NewLogger(cfg *config.LogConfig) (*Logger, error) {

	logger := NewDebugLogger(cfg)

	if cfg.Mode == "prod" || cfg.Mode == "production" {
		level := zap.DebugLevel
		switch strings.ToLower(cfg.Level) {
		case "info":
			level = zap.InfoLevel
		case "warn":
			level = zap.WarnLevel
		case "error":
			level = zap.ErrorLevel
		case "fatal":
			level = zap.FatalLevel
		}
		logger = NewRotatingLogger(
			cfg.Filename,
			100, // 100MB
			10,  // 最多保留10个备份
			7,   // 日志文件最大保存天数（如 7 天）
			true,
			level,
		)
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
