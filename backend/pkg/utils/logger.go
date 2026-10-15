package utils

import (
	"os"
	"path/filepath"

	"github.com/boticz/ai-camera-backend/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var logger *zap.Logger

// InitLogger 依据配置初始化全局 zap 日志。
// format=console 时输出到标准输出（测试环境使用），否则写入滚动文件。
func InitLogger(cfg *config.LogConfig) {
	level := zapcore.InfoLevel
	if cfg != nil {
		switch cfg.Level {
		case "debug":
			level = zapcore.DebugLevel
		case "warn":
			level = zapcore.WarnLevel
		case "error":
			level = zapcore.ErrorLevel
		case "fatal":
			level = zapcore.FatalLevel
		}
	}

	encoderCfg := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	var encoder zapcore.Encoder
	var writeSyncer zapcore.WriteSyncer

	consoleMode := cfg == nil || cfg.Format == "console" || cfg.Output == "console"
	if consoleMode {
		encoder = zapcore.NewConsoleEncoder(encoderCfg)
		writeSyncer = zapcore.AddSync(os.Stdout)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderCfg)

		// 确保日志目录存在
		if dir := filepath.Dir(cfg.Output); dir != "" && dir != "." {
			_ = os.MkdirAll(dir, 0o755)
		}

		writeSyncer = zapcore.AddSync(&lumberjack.Logger{
			Filename:   cfg.Output,
			MaxSize:    cfg.MaxSize,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAge,
		})
	}

	core := zapcore.NewCore(encoder, writeSyncer, level)
	logger = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(0))
	zap.ReplaceGlobals(logger)
}

// Logger 返回全局 logger；未初始化时返回一个可用的 no-op 实例，避免调用方 panic。
func Logger() *zap.Logger {
	if logger == nil {
		return zap.NewNop()
	}
	return logger
}

// Info / Warn / Error / Debug 便捷函数。
func Info(msg string, fields ...zap.Field)  { Logger().Info(msg, fields...) }
func Warn(msg string, fields ...zap.Field)  { Logger().Warn(msg, fields...) }
func Error(msg string, fields ...zap.Field) { Logger().Error(msg, fields...) }
func Debug(msg string, fields ...zap.Field) { Logger().Debug(msg, fields...) }
