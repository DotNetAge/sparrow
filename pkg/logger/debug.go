package logger

import (
	"os"
	"strings"

	"github.com/DotNetAge/sparrow/pkg/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewDebugLogger(cfg *config.LogConfig) *zap.Logger {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",                           // 日志时间字段名
		LevelKey:       "level",                          // 日志级别字段名
		NameKey:        "logger",                         // 日志器名称字段名
		CallerKey:      "caller",                         // 调用者（文件名:行号）字段名
		MessageKey:     "msg",                            // 消息字段名
		StacktraceKey:  "stacktrace",                     // 堆栈跟踪字段名
		LineEnding:     zapcore.DefaultLineEnding,        // 行结束符
		EncodeLevel:    zapcore.CapitalColorLevelEncoder, // 级别带颜色（控制台专用）
		EncodeTime:     zapcore.ISO8601TimeEncoder,       // 时间格式（ISO8601）
		EncodeDuration: zapcore.StringDurationEncoder,    // 耗时格式（字符串）
		EncodeCaller:   zapcore.ShortCallerEncoder,       // 调用者格式（短路径，如 pkg/file.go:123）
	}

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
	// 2. 配置输出目标（控制台）
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)
	writeSyncer := zapcore.Lock(os.Stdout) // 标准输出（控制台）
	core := zapcore.NewCore(consoleEncoder, writeSyncer, level)
	// 3. 创建日志器（带调用者信息、堆栈跟踪）
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	defer logger.Sync() // 确保日志刷新到输出（如文件）
	return logger
}
