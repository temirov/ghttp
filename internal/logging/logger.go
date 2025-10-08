package logging

import (
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	TypeConsole = "CONSOLE"
	TypeJSON    = "JSON"
)

// NormalizeType validates and normalizes the requested logging mode.
func NormalizeType(rawValue string) (string, error) {
	sanitized := strings.ToUpper(strings.TrimSpace(rawValue))
	if sanitized == "" {
		sanitized = TypeConsole
	}
	switch sanitized {
	case TypeConsole, TypeJSON:
		return sanitized, nil
	default:
		return "", fmt.Errorf("unsupported logging type %s", rawValue)
	}
}

// NewLogger creates a logger matching the requested type.
func NewLogger(loggingType string) (*zap.Logger, error) {
	switch loggingType {
	case TypeConsole:
		return NewConsoleLogger(), nil
	case TypeJSON:
		return zap.NewProduction()
	default:
		return nil, fmt.Errorf("unsupported logging type %s", loggingType)
	}
}

// NewConsoleLogger returns a zap logger configured for plain-text output.
func NewConsoleLogger() *zap.Logger {
	encoderConfig := zapcore.EncoderConfig{
		MessageKey:    "msg",
		LevelKey:      "",
		TimeKey:       "",
		NameKey:       "",
		CallerKey:     "",
		FunctionKey:   "",
		StacktraceKey: "",
		LineEnding:    zapcore.DefaultLineEnding,
	}
	core := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderConfig), zapcore.AddSync(os.Stdout), zapcore.InfoLevel)
	return zap.New(core)
}
