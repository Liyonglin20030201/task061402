package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestDispatcher_BelowThreshold(t *testing.T) {
	called := false
	ch := &mockChannel{sendFunc: func(ctx context.Context, event *AlertEvent) error {
		called = true
		return nil
	}}

	d := NewDispatcher([]Channel{ch}, ThresholdConfig{GlobalRiskScore: 60})
	errs := d.Evaluate(context.Background(), 30, nil, "run-1")

	if len(errs) > 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
	if called {
		t.Error("channel should not be called when score is below threshold")
	}
}

func TestDispatcher_AboveThreshold(t *testing.T) {
	var received *AlertEvent
	ch := &mockChannel{sendFunc: func(ctx context.Context, event *AlertEvent) error {
		received = event
		return nil
	}}

	d := NewDispatcher([]Channel{ch}, ThresholdConfig{GlobalRiskScore: 60})
	errs := d.Evaluate(context.Background(), 75, nil, "run-2")

	if len(errs) > 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
	if received == nil {
		t.Fatal("expected channel to be called")
	}
	if received.RiskScore != 75 {
		t.Errorf("expected score 75, got %d", received.RiskScore)
	}
}

func TestDispatcher_CategoryThreshold(t *testing.T) {
	var events []*AlertEvent
	ch := &mockChannel{sendFunc: func(ctx context.Context, event *AlertEvent) error {
		events = append(events, event)
		return nil
	}}

	d := NewDispatcher([]Channel{ch}, ThresholdConfig{
		GlobalRiskScore:    90,
		CategoryThresholds: map[string]int{"replication": 50},
	})

	scores := map[string]int{"replication": 70, "capacity": 20}
	errs := d.Evaluate(context.Background(), 40, scores, "run-3")

	if len(errs) > 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event (category threshold), got %d", len(events))
	}
	if events[0].CheckType != "replication" {
		t.Errorf("expected check type 'replication', got %q", events[0].CheckType)
	}
}

func TestDispatcher_ChannelFailureNonFatal(t *testing.T) {
	failCh := &mockChannel{sendFunc: func(ctx context.Context, event *AlertEvent) error {
		return fmt.Errorf("network error")
	}}
	successCh := &mockChannel{name: "success", sendFunc: func(ctx context.Context, event *AlertEvent) error {
		return nil
	}}

	d := NewDispatcher([]Channel{failCh, successCh}, ThresholdConfig{GlobalRiskScore: 50})
	errs := d.Evaluate(context.Background(), 80, nil, "run-4")

	if len(errs) != 1 {
		t.Errorf("expected 1 error from failed channel, got %d", len(errs))
	}
}

func TestWebhookChannel_Success(t *testing.T) {
	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json content type")
		}
		if r.Header.Get("X-Custom") != "test-header" {
			t.Errorf("expected custom header")
		}
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(200)
	}))
	defer server.Close()

	ch := NewWebhookChannel(server.URL, 5*time.Second, map[string]string{"X-Custom": "test-header"})
	event := &AlertEvent{
		TargetName: "db1",
		CheckType:  "capacity",
		RiskScore:  85,
		RiskLevel:  "CRITICAL",
		Summary:    "disk full",
		OccurredAt: time.Now(),
		RunID:      "run-test",
	}

	err := ch.Send(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedBody["risk_score"] != float64(85) {
		t.Errorf("expected risk_score 85, got %v", receivedBody["risk_score"])
	}
}

func TestWebhookChannel_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	ch := NewWebhookChannel(server.URL, 5*time.Second, nil)
	err := ch.Send(context.Background(), &AlertEvent{RiskScore: 50, OccurredAt: time.Now()})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestSlackChannel_Success(t *testing.T) {
	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(200)
	}))
	defer server.Close()

	ch := NewSlackChannel(server.URL, "#alerts", 5*time.Second)
	err := ch.Send(context.Background(), &AlertEvent{
		TargetName: "db1",
		RiskScore:  90,
		RiskLevel:  "CRITICAL",
		Summary:    "test alert",
		OccurredAt: time.Now(),
		RunID:      "run-slack",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedBody["channel"] != "#alerts" {
		t.Errorf("expected channel '#alerts', got %v", receivedBody["channel"])
	}
}

func TestLogfileChannel_Write(t *testing.T) {
	tmpFile := t.TempDir() + "/alerts.log"
	ch := NewLogfileChannel(tmpFile)

	err := ch.Send(context.Background(), &AlertEvent{
		TargetName: "db1",
		CheckType:  "replication",
		RiskScore:  75,
		RiskLevel:  "HIGH",
		Summary:    "lag detected",
		OccurredAt: time.Now(),
		RunID:      "run-log",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	line := string(content)
	if !strings.Contains(line, "ALERT") {
		t.Error("expected ALERT in log line")
	}
	if !strings.Contains(line, "db1") {
		t.Error("expected target name in log line")
	}
	if !strings.Contains(line, "score=75") {
		t.Error("expected score in log line")
	}
}

type mockChannel struct {
	name     string
	sendFunc func(ctx context.Context, event *AlertEvent) error
}

func (m *mockChannel) Name() string {
	if m.name != "" {
		return m.name
	}
	return "mock"
}

func (m *mockChannel) Send(ctx context.Context, event *AlertEvent) error {
	return m.sendFunc(ctx, event)
}
