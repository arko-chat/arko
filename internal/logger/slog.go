package logger

import (
	"context"
	"log/slog"
	"os"
)

type WailsLogger struct {
	ctx    context.Context
	logger *slog.Logger
}

func New(ctx context.Context) *WailsLogger {
	return &WailsLogger{
		ctx: ctx,
		logger: slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})),
	}
}

func (l *WailsLogger) Print(message string) {
	l.logger.Log(l.ctx, slog.LevelInfo, message)
}

func (l *WailsLogger) Trace(message string) {
	l.logger.Log(l.ctx, slog.LevelDebug-1, message)
}

func (l *WailsLogger) Debug(message string) {
	l.logger.Debug(message)
}

func (l *WailsLogger) Info(message string) {
	l.logger.Info(message)
}

func (l *WailsLogger) Warning(message string) {
	l.logger.Warn(message)
}

func (l *WailsLogger) Error(message string) {
	l.logger.Error(message)
}

func (l *WailsLogger) Fatal(message string) {
	l.logger.Error(message)
	os.Exit(1)
}
