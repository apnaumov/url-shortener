package logger

import (
	"fmt"

	"go.uber.org/zap"
)

// initialize logger with loggerName
func InitializeRootLogger(loggerName string, level string) (*zap.Logger, error) {
	lvl, err := zap.ParseAtomicLevel(level)

	if err != nil {
		return nil, err
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = lvl
	cfg.OutputPaths = []string{
		fmt.Sprintf("%s.log", loggerName),
	}
	zl, err := cfg.Build()
	if err != nil {
		return nil, err
	}

	return zl, nil
}
