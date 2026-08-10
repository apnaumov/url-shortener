package logger

import (
	"errors"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

var (
	LoggerAlreadyInitialized = errors.New("logger already initialized")
	LoggerNotInitialized     = errors.New("logger not initialized")
)

type loggersStorage struct {
	LoggersMap map[string]*zap.Logger
	Mu         sync.Mutex
}

var loggers loggersStorage = loggersStorage{
	LoggersMap: make(map[string]*zap.Logger),
}

// initialize logger with loggerName
func InitializeRootLogger(loggerName string, level string) (*zap.Logger, error) {
	loggers.Mu.Lock()
	if _, ok := loggers.LoggersMap[loggerName]; ok {
		return nil, LoggerAlreadyInitialized
	}
	loggers.Mu.Unlock()

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

	loggers.Mu.Lock()
	defer loggers.Mu.Unlock()

	loggers.LoggersMap[loggerName] = zl

	return zl, nil
}

func GetRootLogger(loggerName string) (*zap.Logger, error) {
	loggers.Mu.Lock()
	defer loggers.Mu.Unlock()

	logger, ok := loggers.LoggersMap[loggerName]

	if !ok {
		return nil, LoggerNotInitialized
	}

	return logger, nil
}
