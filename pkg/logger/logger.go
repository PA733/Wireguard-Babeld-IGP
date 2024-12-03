package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

// NewLogger 创建新的日志记录器
func NewLogger(logPath string, logLevel string) (*zerolog.Logger, error) {
	// 设置日志级别
	level, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// 准备输出写入器
	var writers []io.Writer

	// 总是添加控制台输出
	writers = append(writers, zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "2006-01-02 15:04:05",
	})

	// 如果指定了日志文件，添加文件输出
	if logPath != "" {
		// 确保日志目录存在
		if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
			return nil, fmt.Errorf("creating log directory: %w", err)
		}

		// 配置日志轮转
		fileWriter := &lumberjack.Logger{
			Filename:   logPath,
			MaxSize:    100, // 每个日志文件最大100MB
			MaxBackups: 3,   // 保留3个旧文件
			MaxAge:     28,  // 保留28天
			Compress:   true,
		}

		writers = append(writers, fileWriter)
	}

	// 创建多输出写入器
	mw := zerolog.MultiLevelWriter(writers...)

	// 创建并配置logger
	logger := zerolog.New(mw).With().Timestamp().Logger()

	return &logger, nil
}

// WithComponent 为日志添加组件标识
func WithComponent(logger zerolog.Logger, component string) zerolog.Logger {
	return logger.With().Str("component", component).Logger()
}

// WithFields 为日志添加自定义字段
func WithFields(logger zerolog.Logger, fields map[string]interface{}) zerolog.Logger {
	context := logger.With()
	for k, v := range fields {
		context = context.Interface(k, v)
	}
	return context.Logger()
}
