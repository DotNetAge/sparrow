package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// NewRotatingLogger 创建支持文件轮动的 Zap 日志器
// 参数说明：
// - logPath: 日志文件路径（如 ./logs/app.log）
// - maxSize: 单个日志文件最大尺寸（MB）
// - maxBackups: 最多保留的备份文件数
// - maxAge: 日志文件最大保存天数
// - compress: 是否压缩旧备份
// - logLevel: 日志级别（如 zapcore.DebugLevel）
func NewRotatingLogger(
	logPath string,
	maxSize int,
	maxBackups int,
	maxAge int,
	compress bool,
	logLevel zapcore.Level,
) *zap.Logger {
	// 1. 配置 lumberjack：实现日志轮动
	rotator := &lumberjack.Logger{
		Filename:   logPath,    // 日志文件路径（如 ./logs/app.log）
		MaxSize:    maxSize,    // 单个文件最大尺寸（MB），超过则切割
		MaxBackups: maxBackups, // 最多保留的备份文件数（如 10 个旧文件）
		MaxAge:     maxAge,     // 日志文件最大保存天数（如 7 天）
		Compress:   compress,   // 是否压缩旧备份（.gz 格式）
		LocalTime:  true,       // 备份文件名使用本地时间（默认 UTC）
	}

	// 2. 配置编码器：生产环境用 JSON 格式，开发环境可用控制台格式
	// 此处以生产环境 JSON 格式为例（如需开发环境，替换为 console.Encoder 即可）
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		CallerKey:      "caller",
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,   // 级别大写（INFO/ERROR）
		EncodeTime:     zapcore.ISO8601TimeEncoder,    // 时间格式（2024-05-21T10:30:00.123+0800）
		EncodeCaller:   zapcore.FullCallerEncoder,     // 调用者全路径（便于定位）
		EncodeDuration: zapcore.MillisDurationEncoder, // 耗时（毫秒）
	}
	encoder := zapcore.NewJSONEncoder(encoderConfig)

	// 3. 配置输出目标：同时输出到 轮动文件 + 控制台（可选，开发环境推荐）
	// （1）文件输出：lumberjack 作为 WriteSyncer
	fileSyncer := zapcore.AddSync(rotator)
	// （2）控制台输出：标准输出（可选，生产环境可删除）
	consoleSyncer := zapcore.AddSync(os.Stdout)

	// 4. 组合输出目标（多目标输出）
	multiSyncer := zapcore.NewMultiWriteSyncer(fileSyncer, consoleSyncer)

	// 5. 创建 Zap 核心：编码器 + 多目标输出 + 日志级别
	core := zapcore.NewCore(encoder, multiSyncer, logLevel)

	// 6. 构建日志器：启用调用者信息、错误堆栈
	logger := zap.New(
		core,
		zap.AddCaller(), // 记录调用者（文件名:行号）
		zap.AddStacktrace(zapcore.ErrorLevel),
	) // 错误及以上级别输出堆栈

	return logger
}
