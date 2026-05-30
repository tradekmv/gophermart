// Package logger provides structured logging using zerolog.
package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

var log zerolog.Logger

// Init initializes the global logger with console output.
func Init() *zerolog.Logger {
	zerolog.TimeFieldFormat = time.RFC3339

	output := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "2006-01-02 15:04:05",
	}

	log = zerolog.New(output).
		With().
		Timestamp().
		Caller().
		Logger()

	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	return &log
}

// Get returns the global logger instance.
func Get() *zerolog.Logger {
	return &log
}

// Info logs an info message.
func Info() *zerolog.Event {
	return log.Info()
}

// Error logs an error message.
func Error() *zerolog.Event {
	return log.Error()
}

// Warn logs a warning message.
func Warn() *zerolog.Event {
	return log.Warn()
}

// Debug logs a debug message.
func Debug() *zerolog.Event {
	return log.Debug()
}

// SetLevel sets the global log level.
func SetLevel(level string) {
	switch level {
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}
