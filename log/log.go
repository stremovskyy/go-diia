// Package log contains the small logging abstraction used by go-diia.
package log

import (
	"io"
	"log/slog"
	"os"
	"sync/atomic"
)

// Level controls SDK diagnostic logging.
type Level int32

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelOff
)

// Compatibility aliases used by the other stremovskyy SDKs.
const (
	Debug = LevelDebug
	Info  = LevelInfo
	Warn  = LevelWarn
	Error = LevelError
	Off   = LevelOff
)

// Logger is a concurrency-safe structured logger. SDK messages never include
// credentials, request bodies, callback bodies, or document data.
type Logger struct {
	level atomic.Int32
	slog  *slog.Logger
}

// NewLogger creates a JSON logger writing to stderr.
func NewLogger(component string) *Logger {
	return NewLoggerWithWriter(component, os.Stderr)
}

// NewLoggerWithWriter is useful for applications that route SDK diagnostics.
func NewLoggerWithWriter(component string, w io.Writer) *Logger {
	if w == nil {
		w = io.Discard
	}

	l := &Logger{
		slog: slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: slog.LevelDebug})).
			With("component", component),
	}
	l.level.Store(int32(LevelWarn))

	return l
}

func (l *Logger) SetLevel(level Level) { l.level.Store(int32(level)) }

func (l *Logger) Enabled(level Level) bool {
	current := Level(l.level.Load())

	return current != LevelOff && level >= current
}

func (l *Logger) Debug(msg string, args ...any) {
	if l.Enabled(LevelDebug) {
		l.slog.Debug(msg, args...)
	}
}

func (l *Logger) Info(msg string, args ...any) {
	if l.Enabled(LevelInfo) {
		l.slog.Info(msg, args...)
	}
}

func (l *Logger) Warn(msg string, args ...any) {
	if l.Enabled(LevelWarn) {
		l.slog.Warn(msg, args...)
	}
}

func (l *Logger) Error(msg string, args ...any) {
	if l.Enabled(LevelError) {
		l.slog.Error(msg, args...)
	}
}
