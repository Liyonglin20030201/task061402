package logger

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/Liyonglin20030201/task061402/internal/store"
)

type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

func (l Level) String() string {
	switch l {
	case DEBUG:
		return "debug"
	case INFO:
		return "info"
	case WARN:
		return "warn"
	case ERROR:
		return "error"
	default:
		return "unknown"
	}
}

func ParseLevel(s string) Level {
	switch s {
	case "debug":
		return DEBUG
	case "info":
		return INFO
	case "warn":
		return WARN
	case "error":
		return ERROR
	default:
		return INFO
	}
}

type Logger struct {
	level     Level
	runID     string
	component string
	store     *store.Store
	output    io.Writer
	mu        sync.Mutex
}

func New(level Level, runID string, s *store.Store) *Logger {
	return &Logger{
		level:     level,
		runID:     runID,
		component: "main",
		store:     s,
		output:    os.Stdout,
	}
}

func (l *Logger) WithComponent(component string) *Logger {
	return &Logger{
		level:     l.level,
		runID:     l.runID,
		component: component,
		store:     l.store,
		output:    l.output,
	}
}

func (l *Logger) Debug(msg string, meta ...map[string]interface{}) {
	l.log(DEBUG, msg, meta...)
}

func (l *Logger) Info(msg string, meta ...map[string]interface{}) {
	l.log(INFO, msg, meta...)
}

func (l *Logger) Warn(msg string, meta ...map[string]interface{}) {
	l.log(WARN, msg, meta...)
}

func (l *Logger) Error(msg string, meta ...map[string]interface{}) {
	l.log(ERROR, msg, meta...)
}

func (l *Logger) log(level Level, msg string, meta ...map[string]interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(l.output, "[%s] %s [%s] %s\n", timestamp, level.String(), l.component, msg)

	if l.store != nil {
		var metadata map[string]interface{}
		if len(meta) > 0 {
			metadata = meta[0]
		}
		l.store.SaveLog(&store.OperationLog{
			RunID:     l.runID,
			Level:     level.String(),
			Component: l.component,
			Message:   msg,
			Metadata:  metadata,
		})
	}
}
