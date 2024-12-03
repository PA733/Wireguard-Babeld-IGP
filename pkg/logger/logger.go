package logger

import (
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger 封装了 zerolog.Logger 并包含同步机制
type Logger struct {
	logger zerolog.Logger
	mutex  sync.RWMutex
}

// consoleWriter 用于控制台输出
var consoleWriter = zerolog.ConsoleWriter{
	Out:        os.Stdout,
	TimeFormat: time.RFC3339,
}

// NewLogger 初始化日志系统
func NewLogger(debug bool) *Logger {
	l := &Logger{}

	// 设置全局日志级别
	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	// 初始化 MultiLevelWriter，仅包含控制台输出
	multi := zerolog.MultiLevelWriter(consoleWriter)

	// 创建初始 logger
	l.logger = zerolog.New(multi).
		With().
		Timestamp().
		Caller().
		Logger()

	// 设置全局 logger
	log.Logger = l.logger

	return l
}

// GetLogger 返回带有上下文的日志记录器
func (l *Logger) GetLogger(component string) zerolog.Logger {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	return l.logger.With().
		Str("component", component).
		Logger()
}

// SetLogOutput 设置额外的日志输出（如文件）
func (l *Logger) SetLogOutput(logFilePath string) {
	// 使用 lumberjack 进行日志轮转
	fileWriter := &lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    100, // megabytes
		MaxBackups: 3,
		MaxAge:     28,   // days
		Compress:   true, // 压缩旧文件
	}

	// 创建新的 MultiLevelWriter，包含控制台和文件输出
	multi := zerolog.MultiLevelWriter(consoleWriter, fileWriter)

	l.mutex.Lock()
	defer l.mutex.Unlock()

	// 更新 logger 的输出
	l.logger = zerolog.New(multi).
		With().
		Timestamp().
		Caller().
		Logger()

	// 更新全局 logger
	log.Logger = l.logger
}

// 新增一个 provider
func provideLogger(debug bool, logFile string) *Logger {
	logger := NewLogger(debug)
	if logFile != "" {
		logger.SetLogOutput(logFile)
	}
	return logger
}
