package logger

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestNewConsole(t *testing.T) {
	var buf bytes.Buffer
	logger := NewConsole(&buf, "info")
	if logger == nil {
		t.Fatal("NewConsole() returned nil")
	}

	logger.Info().Msg("hello")
	if !strings.Contains(buf.String(), "hello") {
		t.Errorf("expected output to contain 'hello', got %q", buf.String())
	}
}

func TestNewConsole_InvalidLevel(t *testing.T) {
	// Невалидный level — должно использоваться info.
	var buf bytes.Buffer
	logger := NewConsole(&buf, "bogus")
	logger.Info().Msg("should-appear")
	if !strings.Contains(buf.String(), "should-appear") {
		t.Errorf("expected info message to appear with invalid level, got %q", buf.String())
	}
}

func TestNewConsole_RespectsLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := NewConsole(&buf, "error")
	logger.Info().Msg("hidden")
	if strings.Contains(buf.String(), "hidden") {
		t.Errorf("info should be hidden at error level, got %q", buf.String())
	}

	logger.Error().Msg("visible")
	if !strings.Contains(buf.String(), "visible") {
		t.Errorf("error should be visible at error level, got %q", buf.String())
	}
}

func TestNewNop(t *testing.T) {
	logger := NewNop()
	if logger == nil {
		t.Fatal("NewNop() returned nil")
	}
	// Должен игнорировать всё — нет паники на любых вызовах.
	logger.Info().Msg("ignored")
	logger.Error().Msg("ignored")
}

func TestInit(t *testing.T) {
	// Init() использует stdout; проверим, что возвращает не-nil.
	logger := Init()
	if logger == nil {
		t.Fatal("Init() returned nil")
	}
}

func TestLoggerOutput_Component(t *testing.T) {
	// Проверяем, что .With().Str("component", ...).Logger() работает.
	var buf bytes.Buffer
	base := NewConsole(&buf, "info")
	handlerLogger := base.With().Str("component", "handler").Logger()

	handlerLogger.Info().Msg("test")

	if !strings.Contains(buf.String(), "component") {
		t.Errorf("expected output to contain 'component', got %q", buf.String())
	}
	if !strings.Contains(buf.String(), "handler") {
		t.Errorf("expected output to contain 'handler', got %q", buf.String())
	}
}

// Доп. проверка: zerolog.Logger можно использовать без нашего пакета
// (для совместимости).
func TestLoggerHelpers_PlainZerolog(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	logger.Info().Msg("plain")

	if !strings.Contains(buf.String(), "plain") {
		t.Errorf("expected output to contain 'plain', got %q", buf.String())
	}
}
