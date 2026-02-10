package logger

import (
	"fmt"
	"io"
	"log/slog"
	"networking/config"
	"os"
)

type Logger struct {
	*slog.Logger
}

const (
	local = "local"
	dev   = "dev"
	prod  = "prod"
)

func NewLogger(acfg config.App) *Logger {
	var log *slog.Logger
	env := acfg.Env
	writer := io.Writer(os.Stdout)
	if acfg.LogPath != "" {
		logFile, err := os.OpenFile(acfg.LogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			panic(fmt.Errorf("failed to open log file: %w", err))
		}
		writer = io.Writer(logFile)
	}
	switch env {
	case local:
		log = slog.New(slog.NewTextHandler(writer, &slog.HandlerOptions{Level: slog.LevelDebug}))
	case dev:
		log = slog.New(slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: slog.LevelDebug}))
	case prod:
		log = slog.New(slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: slog.LevelInfo}))
	default:
		log = slog.New(slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}

	logger := &Logger{
		log.With(
			slog.String("env", env),
			slog.String("app", acfg.Name),
			slog.String("version", acfg.Version),
		),
	}
	return logger
}

func (l *Logger) Info(msg string, args ...any) {
	l.Logger.Info(msg, args...)
}

func (l *Logger) Error(msg string, args ...any) {
	l.Logger.Error(msg, args...)
}

func (l *Logger) Debug(msg string, args ...any) {
	l.Logger.Debug(msg, args...)
}

func (l *Logger) AddOp(op string) *Logger {
	return &Logger{l.Logger.With(slog.String("op", op))}
}

func Err(err error) slog.Attr {
	return slog.Any("error", err)
}

func Attr(key string, val any) slog.Attr {
	return slog.Any(key, val)
}

func NewLogData(attrs ...any) []any {
	ld := make([]any, 0, len(attrs))
	ld = append(ld, attrs...)
	return ld
}
