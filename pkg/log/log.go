package log

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// Level represents a log severity level.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

var levelNames = map[Level]string{
	LevelDebug: "DEBUG",
	LevelInfo:  "INFO",
	LevelWarn:  "WARN",
	LevelError: "ERROR",
}

var levelFromString = map[string]Level{
	"debug": LevelDebug,
	"info":  LevelInfo,
	"warn":  LevelWarn,
	"error": LevelError,
}

var (
	globalLevel Level
	mu          sync.Mutex
	startTime   time.Time
)

func init() {
	startTime = time.Now()
	globalLevel = LevelInfo

	if env := os.Getenv("SDT_LOG_LEVEL"); env != "" {
		if l, ok := levelFromString[strings.ToLower(env)]; ok {
			globalLevel = l
		}
	}
}

// SetLevel sets the global log level.
func SetLevel(l Level) {
	mu.Lock()
	defer mu.Unlock()
	globalLevel = l
}

// GetLevel returns the current global log level.
func GetLevel() Level {
	mu.Lock()
	defer mu.Unlock()
	return globalLevel
}

func logf(level Level, component, format string, args ...interface{}) {
	mu.Lock()
	current := globalLevel
	mu.Unlock()

	if level < current {
		return
	}

	elapsed := time.Since(startTime)
	msg := fmt.Sprintf(format, args...)
	// Format: elapsed_seconds LEVEL [COMPONENT] message
	fmt.Fprintf(os.Stderr, "%7.1fs %-5s [%-6s] %s\n", elapsed.Seconds(), levelNames[level], component, msg)
}

// Debugf logs a debug-level message.
func Debugf(component, format string, args ...interface{}) {
	logf(LevelDebug, component, format, args...)
}

// Infof logs an info-level message.
func Infof(component, format string, args ...interface{}) {
	logf(LevelInfo, component, format, args...)
}

// Warnf logs a warn-level message.
func Warnf(component, format string, args ...interface{}) {
	logf(LevelWarn, component, format, args...)
}

// Errorf logs an error-level message.
func Errorf(component, format string, args ...interface{}) {
	logf(LevelError, component, format, args...)
}
