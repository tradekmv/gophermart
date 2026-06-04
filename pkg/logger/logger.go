// Package logger provides structured logging using zerolog.
//
// Logger создаётся через Init() / NewConsole() и явно прокидывается в
// компоненты через DI — НЕЛЬЗЯ обращаться к глобальному состоянию пакета.
package logger

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// Init создаёт root-логгер с выводом в stdout (консольный writer)
// и level=Info. Удобно вызывать один раз в main() и прокидывать
// дочерние логгеры в компоненты:
//
//	appLogger := logger.Init()
//	handlerLogger := appLogger.With().Str("component", "handler").Logger()
//	NewBalanceHandler(svc, &handlerLogger, ...)
//
// В тестах использовать NewConsole("error") + &log или NewNop().
func Init() *zerolog.Logger {
	return NewConsole(os.Stdout, "info")
}

// NewConsole создаёт zerolog.Logger с консольным writer'ом и level.
// Если level невалиден — используется info. Передавай свой io.Writer для тестов.
func NewConsole(w io.Writer, level string) *zerolog.Logger {
	zerolog.TimeFieldFormat = time.RFC3339

	lvl, err := zerolog.ParseLevel(strings.ToLower(level))
	if err != nil || lvl == zerolog.NoLevel {
		lvl = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(lvl)

	output := zerolog.ConsoleWriter{
		Out:        w,
		TimeFormat: "2006-01-02 15:04:05",
	}

	logger := zerolog.New(output).
		With().
		Timestamp().
		Caller().
		Logger().
		Level(lvl)

	return &logger
}

// NewNop создаёт "no-op" логгер, который игнорирует все записи.
// Используется в тестах компонентов, чтобы не возиться с буфером вывода.
func NewNop() *zerolog.Logger {
	nop := zerolog.Nop()
	return &nop
}
