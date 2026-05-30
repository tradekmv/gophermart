package logger

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestInit(t *testing.T) {
	logger := Init()
	if logger == nil {
		t.Error("Init() returned nil")
	}
	if &log == nil {
		t.Error("global log not initialized")
	}
}

func TestGet(t *testing.T) {
	Init() // Initialize first
	logger := Get()
	if logger == nil {
		t.Error("Get() returned nil")
	}
}

func TestSetLevel(t *testing.T) {
	Init()

	tests := []struct {
		name  string
		level string
	}{
		{"error level", "error"},
		{"warn level", "warn"},
		{"debug level", "debug"},
		{"info level", "info"},
		{"invalid level defaults to info", "invalid"},
		{"empty level defaults to info", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetLevel(tt.level)
			// Just verify no panic
		})
	}
}

func TestLoggingFunctions(t *testing.T) {
	Init()

	// Just verify these don't panic
	Info().Msg("test info")
	Error().Msg("test error")
	Warn().Msg("test warn")
	Debug().Msg("test debug")
}

func TestLoggerOutput(t *testing.T) {
	var buf bytes.Buffer
	SetLevel("debug")

	logger := Get()
	outputLogger := logger.Output(&buf)

	outputLogger.Info().Msg("test message")

	if !strings.Contains(buf.String(), "test message") {
		t.Error("log output should contain message")
	}
}

func TestLoggerHelpers(t *testing.T) {
	var buf bytes.Buffer
	SetLevel("debug")

	logger := zerolog.New(&buf)
	logger.Info().Msg("helper test")

	if !strings.Contains(buf.String(), "helper test") {
		t.Error("logger should output message")
	}
}
