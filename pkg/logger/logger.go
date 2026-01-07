package logger

import (
	"go.uber.org/zap"
)

type Config struct {
	LogLevel    string `mapstructure:"log_level"`
	ServiceName string `mapstructure:"service_name"`
}

func NewLogger(config Config) (*zap.Logger, error) {
	var cfg zap.Config
	switch config.LogLevel {
	case "debug":
		cfg = zap.NewDevelopmentConfig()
		cfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		cfg = zap.NewProductionConfig()
		cfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		cfg = zap.NewProductionConfig()
		cfg.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error", "fatal":
		cfg = zap.NewProductionConfig()
		cfg.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		cfg = zap.NewProductionConfig()
		cfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	cfg.OutputPaths = []string{"stderr"}
	logger, err := cfg.Build()
	if err != nil {
		return nil, err
	}

	logger = logger.With(zap.String("service", config.ServiceName))

	return logger, nil
}
