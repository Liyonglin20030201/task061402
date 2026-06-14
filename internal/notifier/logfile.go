package notifier

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"
)

type LogfileChannel struct {
	path string
	mu   sync.Mutex
}

func NewLogfileChannel(path string) *LogfileChannel {
	return &LogfileChannel{path: path}
}

func (l *LogfileChannel) Name() string { return "logfile" }

func (l *LogfileChannel) Send(ctx context.Context, event *AlertEvent) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open alert log file: %w", err)
	}
	defer f.Close()

	line := fmt.Sprintf("[%s] %s target=%s check=%s score=%d level=%s summary=%q run_id=%s\n",
		time.Now().Format("2006-01-02 15:04:05"),
		"ALERT",
		event.TargetName,
		event.CheckType,
		event.RiskScore,
		event.RiskLevel,
		event.Summary,
		event.RunID,
	)

	if _, err := f.WriteString(line); err != nil {
		return fmt.Errorf("failed to write alert log: %w", err)
	}

	return nil
}
