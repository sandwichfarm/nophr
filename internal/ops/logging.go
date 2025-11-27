package ops

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/sandwichfarm/nophr/internal/config"
)

// Logger is a structured logger wrapper
type Logger struct {
	*slog.Logger
	level  slog.Level
	format string
}

// NewLogger creates a new structured logger based on config
func NewLogger(cfg *config.Logging) *Logger {
	var level slog.Level
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Customize timestamp format
			if a.Key == slog.TimeKey {
				if t, ok := a.Value.Any().(time.Time); ok {
					a.Value = slog.StringValue(t.Format(time.RFC3339))
				}
			}
			return a
		},
	}

	var handler slog.Handler
	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return &Logger{
		Logger: slog.New(handler),
		level:  level,
		format: cfg.Format,
	}
}

// NewLoggerWithWriter creates a logger with a custom writer
func NewLoggerWithWriter(cfg *config.Logging, w io.Writer) *Logger {
	var level slog.Level
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(w, opts)
	} else {
		handler = slog.NewTextHandler(w, opts)
	}

	return &Logger{
		Logger: slog.New(handler),
		level:  level,
		format: cfg.Format,
	}
}

// WithComponent adds a component field to all log messages
func (l *Logger) WithComponent(component string) *Logger {
	return &Logger{
		Logger: l.Logger.With("component", component),
		level:  l.level,
		format: l.format,
	}
}

// WithFields adds custom fields to the logger
func (l *Logger) WithFields(fields ...any) *Logger {
	return &Logger{
		Logger: l.Logger.With(fields...),
		level:  l.level,
		format: l.format,
	}
}

// IsDebugEnabled returns true if debug logging is enabled
func (l *Logger) IsDebugEnabled() bool {
	return l.level <= slog.LevelDebug
}

// Component-specific logger helpers

// LogStorageOperation logs a storage operation
func (l *Logger) LogStorageOperation(op string, duration time.Duration, err error) {
	if err != nil {
		l.Error("storage operation failed",
			"operation", op,
			"duration_ms", duration.Milliseconds(),
			"error", err)
	} else {
		l.Debug("storage operation completed",
			"operation", op,
			"duration_ms", duration.Milliseconds())
	}
}

// LogRelayConnection logs a relay connection event
func (l *Logger) LogRelayConnection(relay string, connected bool, err error) {
	if err != nil {
		l.Warn("relay connection failed",
			"relay", relay,
			"error", err)
	} else if connected {
		l.Info("relay connected",
			"relay", relay)
	} else {
		l.Info("relay disconnected",
			"relay", relay)
	}
}

// LogSyncProgress logs sync engine progress
func (l *Logger) LogSyncProgress(relay string, kind int, count int, cursor int64) {
	l.Debug("sync progress",
		"relay", relay,
		"kind", kind,
		"events", count,
		"cursor", cursor)
}

// LogProtocolRequest logs a protocol server request
func (l *Logger) LogProtocolRequest(protocol string, selector string, duration time.Duration, err error) {
	if err != nil {
		l.Error("protocol request failed",
			"protocol", protocol,
			"selector", selector,
			"duration_ms", duration.Milliseconds(),
			"error", err)
	} else {
		l.Info("protocol request",
			"protocol", protocol,
			"selector", selector,
			"duration_ms", duration.Milliseconds())
	}
}

// LogCacheOperation logs a cache operation
func (l *Logger) LogCacheOperation(op string, key string, hit bool) {
	l.Debug("cache operation",
		"operation", op,
		"key", key,
		"hit", hit)
}

// LogAggregateUpdate logs an aggregate computation
func (l *Logger) LogAggregateUpdate(eventID string, kind int, replyCount, reactionCount, zapCount int) {
	l.Debug("aggregate updated",
		"event_id", eventID,
		"kind", kind,
		"replies", replyCount,
		"reactions", reactionCount,
		"zaps", zapCount)
}

// LogRetentionPrune logs a retention pruning operation
func (l *Logger) LogRetentionPrune(deletedCount int, duration time.Duration, err error) {
	if err != nil {
		l.Error("retention pruning failed",
			"deleted", deletedCount,
			"duration_ms", duration.Milliseconds(),
			"error", err)
	} else {
		l.Info("retention pruning completed",
			"deleted", deletedCount,
			"duration_ms", duration.Milliseconds())
	}
}

// LogBackupOperation logs a backup operation
func (l *Logger) LogBackupOperation(op string, path string, sizeBytes int64, err error) {
	if err != nil {
		l.Error("backup operation failed",
			"operation", op,
			"path", path,
			"error", err)
	} else {
		l.Info("backup operation completed",
			"operation", op,
			"path", path,
			"size_bytes", sizeBytes)
	}
}

// LogStartup logs application startup information
func (l *Logger) LogStartup(version, commit string, config map[string]interface{}) {
	l.Info("nophr starting",
		"version", version,
		"commit", commit,
		"config", config)
}

// LogShutdown logs application shutdown
func (l *Logger) LogShutdown(reason string) {
	l.Info("nophr shutting down",
		"reason", reason)
}

// LogPanic logs a panic with stack trace
func (l *Logger) LogPanic(recovered interface{}, stack string) {
	l.Error("panic recovered",
		"panic", fmt.Sprintf("%v", recovered),
		"stack", stack)
}

// Default logger configuration
var defaultLogger *Logger

func init() {
	// Create a default logger for early startup
	defaultLogger = NewLogger(&config.Logging{
		Level:  "info",
		Format: "text",
	})
}

// Default returns the default logger
func Default() *Logger {
	return defaultLogger
}

// SetDefault sets the default logger
func SetDefault(l *Logger) {
	defaultLogger = l
}

// Helper functions for common logging patterns

// Info logs an info message
func Info(msg string, fields ...any) {
	defaultLogger.Info(msg, fields...)
}

// Debug logs a debug message
func Debug(msg string, fields ...any) {
	defaultLogger.Debug(msg, fields...)
}

// Warn logs a warning message
func Warn(msg string, fields ...any) {
	defaultLogger.Warn(msg, fields...)
}

// Error logs an error message
func Error(msg string, fields ...any) {
	defaultLogger.Error(msg, fields...)
}
