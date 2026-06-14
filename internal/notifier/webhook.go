package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type WebhookChannel struct {
	url     string
	timeout time.Duration
	headers map[string]string
}

func NewWebhookChannel(url string, timeout time.Duration, headers map[string]string) *WebhookChannel {
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	return &WebhookChannel{
		url:     url,
		timeout: timeout,
		headers: headers,
	}
}

func (w *WebhookChannel) Name() string { return "webhook" }

func (w *WebhookChannel) Send(ctx context.Context, event *AlertEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal alert event: %w", err)
	}

	sendCtx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(sendCtx, http.MethodPost, w.url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range w.headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}
