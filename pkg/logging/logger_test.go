package logging_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/temirov/ghttp/pkg/logging"
)

const (
	consoleMessageKey           = "msg"
	consolePortionSeparator     = "\n"
	jsonMessageKey              = "msg"
	jsonLevelKey                = "level"
	jsonErrorKey                = "error"
	jsonFieldKeyStatus          = "status"
	jsonFieldKeyPath            = "path"
	jsonFieldKeyRemote          = "remote"
	consoleFieldTemplate        = "%s=\"%s\""
	consoleErrorTemplate        = "error=\"%s\""
	expectedInfoLevelValue      = "info"
	expectedErrorLevelValue     = "error"
	consoleLogMessage           = "serving http"
	consoleErrorMessage         = "shutdown failed"
	consoleFieldNameDirectory   = "directory"
	consoleFieldValueDirectory  = "/tmp/site"
	jsonLogMessage              = "request completed"
	jsonErrorMessage            = "request failed"
	jsonFieldValueStatus        = 200
	jsonFieldValuePath          = "/index.html"
	jsonFieldValueRemoteAddress = "127.0.0.1"
	jsonStructuredErrorValue    = "boom"
)

func TestNormalizeTypeHandlesVariants(t *testing.T) {
	testCases := []struct {
		testName     string
		rawValue     string
		expectedType string
		expectErr    bool
	}{
		{
			testName:     "EmptyValueDefaultsToConsole",
			rawValue:     "",
			expectedType: logging.TypeConsole,
		},
		{
			testName:     "WhitespaceDefaultsToConsole",
			rawValue:     "   ",
			expectedType: logging.TypeConsole,
		},
		{
			testName:     "LowercaseConsole",
			rawValue:     "console",
			expectedType: logging.TypeConsole,
		},
		{
			testName:     "MixedCaseJSON",
			rawValue:     " JsOn ",
			expectedType: logging.TypeJSON,
		},
		{
			testName:  "UnsupportedType",
			rawValue:  "text",
			expectErr: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.testName, func(t *testing.T) {
			actualType, err := logging.NormalizeType(testCase.rawValue)
			if testCase.expectErr {
				if err == nil {
					t.Fatalf("expected error for value %q", testCase.rawValue)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalize returned unexpected error: %v", err)
			}
			if actualType != testCase.expectedType {
				t.Fatalf("expected %s, got %s", testCase.expectedType, actualType)
			}
		})
	}
}

func TestServiceLogsConsoleFriendlyMessages(t *testing.T) {
	logBuffer := &bytes.Buffer{}
	consoleEncoderConfig := zapcore.EncoderConfig{
		MessageKey: consoleMessageKey,
		LineEnding: zapcore.DefaultLineEnding,
	}
	consoleCore := zapcore.NewCore(zapcore.NewConsoleEncoder(consoleEncoderConfig), zapcore.AddSync(logBuffer), zapcore.InfoLevel)
	consoleLogger := zap.New(consoleCore)
	loggingService, err := logging.NewServiceWithLogger(logging.TypeConsole, consoleLogger)
	if err != nil {
		t.Fatalf("failed to create logging service: %v", err)
	}

	loggingService.Info(consoleLogMessage, logging.String(consoleFieldNameDirectory, consoleFieldValueDirectory))
	consoleError := errors.New(jsonStructuredErrorValue)
	loggingService.Error(consoleErrorMessage, consoleError)
	if syncErr := loggingService.Sync(); syncErr != nil {
		t.Fatalf("failed to sync console logger: %v", syncErr)
	}

	consoleLogLines := strings.Split(strings.TrimSuffix(logBuffer.String(), consolePortionSeparator), consolePortionSeparator)
	if len(consoleLogLines) != 2 {
		t.Fatalf("expected two console log lines, got %d", len(consoleLogLines))
	}

	infoLine := consoleLogLines[0]
	expectedField := fmt.Sprintf(consoleFieldTemplate, consoleFieldNameDirectory, consoleFieldValueDirectory)
	if !strings.Contains(infoLine, consoleLogMessage) {
		t.Fatalf("info line missing message %q: %s", consoleLogMessage, infoLine)
	}
	if !strings.Contains(infoLine, expectedField) {
		t.Fatalf("info line missing field %q: %s", expectedField, infoLine)
	}

	errorLine := consoleLogLines[1]
	expectedErrorField := fmt.Sprintf(consoleErrorTemplate, consoleError.Error())
	if !strings.Contains(errorLine, consoleErrorMessage) {
		t.Fatalf("error line missing message %q: %s", consoleErrorMessage, errorLine)
	}
	if !strings.Contains(errorLine, expectedErrorField) {
		t.Fatalf("error line missing error field %q: %s", expectedErrorField, errorLine)
	}
}

func TestServiceLogsStructuredMessages(t *testing.T) {
	logBuffer := &bytes.Buffer{}
	jsonEncoderConfig := zap.NewProductionEncoderConfig()
	jsonEncoderConfig.MessageKey = jsonMessageKey
	jsonEncoderConfig.LevelKey = jsonLevelKey
	jsonEncoderConfig.TimeKey = ""
	jsonEncoderConfig.CallerKey = ""
	jsonEncoderConfig.StacktraceKey = ""
	jsonCore := zapcore.NewCore(zapcore.NewJSONEncoder(jsonEncoderConfig), zapcore.AddSync(logBuffer), zapcore.InfoLevel)
	jsonLogger := zap.New(jsonCore)
	loggingService, err := logging.NewServiceWithLogger(logging.TypeJSON, jsonLogger)
	if err != nil {
		t.Fatalf("failed to create structured logging service: %v", err)
	}

	loggingService.Info(jsonLogMessage,
		logging.Int(jsonFieldKeyStatus, jsonFieldValueStatus),
		logging.String(jsonFieldKeyPath, jsonFieldValuePath),
		logging.String(jsonFieldKeyRemote, jsonFieldValueRemoteAddress),
	)
	structuredError := errors.New(jsonStructuredErrorValue)
	loggingService.Error(jsonErrorMessage, structuredError)
	if syncErr := loggingService.Sync(); syncErr != nil {
		t.Fatalf("failed to sync structured logger: %v", syncErr)
	}

	structuredLogLines := strings.Split(strings.TrimSpace(logBuffer.String()), consolePortionSeparator)
	if len(structuredLogLines) != 2 {
		t.Fatalf("expected two structured log lines, got %d", len(structuredLogLines))
	}

	infoEntry := map[string]any{}
	if err := json.Unmarshal([]byte(structuredLogLines[0]), &infoEntry); err != nil {
		t.Fatalf("failed to parse info entry: %v", err)
	}
	assertJSONField(t, infoEntry, jsonMessageKey, jsonLogMessage)
	assertJSONField(t, infoEntry, jsonLevelKey, expectedInfoLevelValue)
	assertJSONField(t, infoEntry, jsonFieldKeyStatus, float64(jsonFieldValueStatus))
	assertJSONField(t, infoEntry, jsonFieldKeyPath, jsonFieldValuePath)
	assertJSONField(t, infoEntry, jsonFieldKeyRemote, jsonFieldValueRemoteAddress)

	errorEntry := map[string]any{}
	if err := json.Unmarshal([]byte(structuredLogLines[1]), &errorEntry); err != nil {
		t.Fatalf("failed to parse error entry: %v", err)
	}
	assertJSONField(t, errorEntry, jsonMessageKey, jsonErrorMessage)
	assertJSONField(t, errorEntry, jsonLevelKey, expectedErrorLevelValue)
	assertJSONField(t, errorEntry, jsonErrorKey, structuredError.Error())
}

func assertJSONField(t *testing.T, entry map[string]any, key string, expected any) {
	t.Helper()
	value, exists := entry[key]
	if !exists {
		t.Fatalf("missing key %q", key)
	}
	if value != expected {
		t.Fatalf("expected %q for key %q, got %v", expected, key, value)
	}
}
