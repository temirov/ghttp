package logging

import (
	"fmt"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	TypeConsole = "CONSOLE"
	TypeJSON    = "JSON"
)

// Field represents a logging attribute.
type Field struct {
	Key   string
	Value any
}

// String creates a string Field.
func String(key string, value string) Field {
	return Field{Key: key, Value: value}
}

// Strings creates a []string Field.
func Strings(key string, value []string) Field {
	return Field{Key: key, Value: value}
}

// Int creates an int Field.
func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

// Duration creates a time.Duration Field.
func Duration(key string, value time.Duration) Field {
	return Field{Key: key, Value: value}
}

// ErrorField creates an error Field using the key "error".
func ErrorField(err error) Field {
	return Field{Key: "error", Value: err}
}

// Any creates a Field from any value.
func Any(key string, value any) Field {
	return Field{Key: key, Value: value}
}

// NormalizeType validates and normalizes a logging type string.
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

// Service provides logging capabilities with console and JSON modes.
type Service struct {
	loggingType string
	logger      *zap.Logger
}

// NewService constructs a logging Service using the provided type.
func NewService(loggingType string) (*Service, error) {
	normalized, err := NormalizeType(loggingType)
	if err != nil {
		return nil, err
	}
	logger, err := newZapLogger(normalized)
	if err != nil {
		return nil, err
	}
	return &Service{loggingType: normalized, logger: logger}, nil
}

// NewServiceWithLogger constructs a Service using an existing zap logger.
func NewServiceWithLogger(loggingType string, logger *zap.Logger) (*Service, error) {
	normalized, err := NormalizeType(loggingType)
	if err != nil {
		return nil, err
	}
	if logger == nil {
		return nil, fmt.Errorf("logger must not be nil")
	}
	return &Service{loggingType: normalized, logger: logger}, nil
}

// NewTestService constructs a Service backed by zap.NewNop for testing.
func NewTestService(loggingType string) *Service {
	normalized, err := NormalizeType(loggingType)
	if err != nil {
		normalized = TypeConsole
	}
	service, err := NewServiceWithLogger(normalized, zap.NewNop())
	if err != nil {
		panic(err)
	}
	return service
}

// Type returns the current logging type.
func (service *Service) Type() string {
	return service.loggingType
}

// Info writes an informational message.
func (service *Service) Info(message string, fields ...Field) {
	service.log(zapcore.InfoLevel, message, nil, fields...)
}

// Error writes an error message with the provided error.
func (service *Service) Error(message string, err error, fields ...Field) {
	service.log(zapcore.ErrorLevel, message, err, fields...)
}

// Sync flushes buffered log entries.
func (service *Service) Sync() error {
	return service.logger.Sync()
}

func (service *Service) log(level zapcore.Level, message string, err error, fields ...Field) {
	if err != nil {
		fields = append(fields, ErrorField(err))
	}
	if service.loggingType == TypeConsole {
		formatted := formatConsoleMessage(message, fields)
		service.logger.Info(formatted)
		return
	}
	zapFields := make([]zap.Field, 0, len(fields))
	for _, field := range fields {
		zapFields = append(zapFields, convertToZapField(field))
	}
	switch level {
	case zapcore.ErrorLevel:
		service.logger.Error(message, zapFields...)
	default:
		service.logger.Info(message, zapFields...)
	}
}

func convertToZapField(field Field) zap.Field {
	switch value := field.Value.(type) {
	case error:
		return zap.NamedError(field.Key, value)
	case []string:
		return zap.Strings(field.Key, value)
	case string:
		return zap.String(field.Key, value)
	case time.Duration:
		return zap.Duration(field.Key, value)
	case int:
		return zap.Int(field.Key, value)
	default:
		return zap.Any(field.Key, value)
	}
}

func formatConsoleMessage(message string, fields []Field) string {
	if len(fields) == 0 {
		return message
	}
	var builder strings.Builder
	builder.WriteString(message)
	for _, field := range fields {
		builder.WriteString(" ")
		builder.WriteString(field.Key)
		builder.WriteString("=")
		builder.WriteString(formatConsoleValue(field.Value))
	}
	return builder.String()
}

func formatConsoleValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return "<nil>"
	case string:
		return fmt.Sprintf("\"%s\"", typed)
	case []string:
		return fmt.Sprintf("[%s]", strings.Join(typed, ","))
	case time.Duration:
		return fmt.Sprintf("\"%s\"", typed)
	case error:
		return fmt.Sprintf("\"%s\"", typed.Error())
	default:
		return fmt.Sprint(typed)
	}
}

func newZapLogger(loggingType string) (*zap.Logger, error) {
	switch loggingType {
	case TypeConsole:
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
		return zap.New(core), nil
	case TypeJSON:
		return zap.NewProduction()
	default:
		return nil, fmt.Errorf("unsupported logging type %s", loggingType)
	}
}
