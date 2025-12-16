package worker

import (
	"fmt"
	"log/slog"
)

type LoggerAdapter struct {
	logger *slog.Logger
}

func NewLoggerAdapter(logger *slog.Logger) *LoggerAdapter {
	return &LoggerAdapter{logger: logger}
}

func (l *LoggerAdapter) Debug(args ...any) {
	l.logger.Debug(fmt.Sprint(args...))
}

func (l *LoggerAdapter) Info(args ...any) {
	l.logger.Info(fmt.Sprint(args...))
}

func (l *LoggerAdapter) Warn(args ...any) {
	l.logger.Warn(fmt.Sprint(args...))
}

func (l *LoggerAdapter) Error(args ...any) {
	l.logger.Error(fmt.Sprint(args...))
}

func (l *LoggerAdapter) Fatal(args ...any) {
	// calling Error then panic, or just Error as we can't easily fatal with slog without os.Exit
	l.logger.Error("FATAL: " + fmt.Sprint(args...))
	// asynq might expect Fatal to panic or exit, but standard log.Fatal does exit.
	// For safety, we'll just log Error here to avoid killing the app if asynq misuses it,
	// though asynq usually uses Fatal for unrecoverable startup errors.
	panic(fmt.Sprint(args...))
}
