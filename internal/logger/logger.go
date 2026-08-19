package logger

import (
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
)

var logDirectory string = "."

func SetLogDirectory(logDir string) error {
	logDirectory = logDir

	err := os.MkdirAll(logDir, 0755)
	return err
}

// initialize logger with loggerName
func InitializeRootLogger(loggerName string, level string) (*zap.Logger, error) {
	lvl, err := zap.ParseAtomicLevel(level)

	if err != nil {
		return nil, err
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = lvl
	cfg.OutputPaths = []string{
		filepath.Join(logDirectory, fmt.Sprintf("%s.log", loggerName)),
	}
	zl, err := cfg.Build()
	if err != nil {
		return nil, err
	}

	return zl, nil
}
