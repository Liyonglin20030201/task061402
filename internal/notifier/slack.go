package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type SlackChannel struct {
	webhookURL string
	channel    string
	timeout    time.Duration
}

func NewSlackChannel(webhookURL, channel string, timeout time.Duration) *SlackChannel {
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	return &SlackChannel{
		webhookURL: webhookURL,
		channel:    channel,
		timeout:    timeout,
	}
}

func (s *SlackChannel) Name() string { return "slack" }

func (s *SlackChannel) Send(ctx context.Context, event *AlertEvent) error {
	color := "#36a64f"
	switch {
	case event.RiskScore >= 80:
		color = "#ff0000"
	case event.RiskScore >= 60:
		color = "#ff8c00"
	case event.RiskScore >= 40:
		color = "#ffd700"
	case event.RiskScore >= 20:
		color = "#87ceeb"
	}

	attachment := map[string]interface{}{
		"color":     color,
		"title":     fmt.Sprintf("dbinspect Alert: %s", event.RiskLevel),
		"text":      event.Summary,
		"footer":    fmt.Sprintf("Run ID: %s", event.RunID),
		"ts":        event.OccurredAt.Unix(),
		"fields": []map[string]interface{}{
			{"title": "Target", "value": event.TargetName, "short": true},
			{"title": "Check", "value": event.CheckType, "short": true},
			{"title": "Risk Score", "value": fmt.Sprintf("%d/100", event.RiskScore), "short": true},
			{"title": "Level", "value": event.RiskLevel, "short": true},
		},
	}

	payload := map[string]interface{}{
		"attachments": []interface{}{attachment},
	}
	if s.channel != "" {
		payload["channel"] = s.channel
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal slack payload: %w", err)
	}

	sendCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(sendCtx, http.MethodPost, s.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create slack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("slack webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("slack webhook returned status %d", resp.StatusCode)
	}

	return nil
}
