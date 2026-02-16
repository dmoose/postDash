package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestNotifierMatchAndFire(t *testing.T) {
	var mu sync.Mutex
	var received []map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var evt map[string]any
		json.Unmarshal(body, &evt)
		mu.Lock()
		received = append(received, evt)
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer server.Close()

	rules := []NotifyRule{
		{
			Name:  "errors only",
			Match: MatchRule{Level: "error"},
			Webhook: &Webhook{
				URL: server.URL,
			},
		},
	}

	notifier, err := NewNotifier(rules)
	if err != nil {
		t.Fatal(err)
	}

	// Should NOT fire (info level)
	notifier.Check(Event{Source: "test", Level: "info", Action: "ok"})

	// Should fire (error level)
	notifier.Check(Event{Source: "test", Level: "error", Action: "db_down"})

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 {
		t.Fatalf("expected 1 webhook call, got %d", len(received))
	}
	if received[0]["action"] != "db_down" {
		t.Errorf("expected action db_down, got %v", received[0]["action"])
	}
}

func TestNotifierMultipleRules(t *testing.T) {
	var mu sync.Mutex
	var count int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		count++
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer server.Close()

	rules := []NotifyRule{
		{
			Name:    "errors",
			Match:   MatchRule{Level: "error"},
			Webhook: &Webhook{URL: server.URL},
		},
		{
			Name:    "ci events",
			Match:   MatchRule{Source: "ci"},
			Webhook: &Webhook{URL: server.URL},
		},
	}

	notifier, _ := NewNotifier(rules)

	// Matches both rules
	notifier.Check(Event{Source: "ci", Level: "error", Action: "build_failed"})

	mu.Lock()
	defer mu.Unlock()
	if count != 2 {
		t.Errorf("expected 2 webhook calls (matched 2 rules), got %d", count)
	}
}

func TestMatchRuleANDLogic(t *testing.T) {
	rule := MatchRule{Source: "app", Level: "error"}

	tests := []struct {
		event   Event
		matches bool
	}{
		{Event{Source: "app", Level: "error"}, true},
		{Event{Source: "app", Level: "info"}, false},
		{Event{Source: "other", Level: "error"}, false},
		{Event{Source: "app", Level: "error", Channel: "ops"}, true}, // extra fields don't matter
	}

	for _, tt := range tests {
		if rule.Matches(tt.event) != tt.matches {
			t.Errorf("event %+v: expected match=%v", tt.event, tt.matches)
		}
	}
}

func TestConfigLoadAndValidation(t *testing.T) {
	// Rule with no target should fail
	_, err := NewNotifier([]NotifyRule{
		{Name: "bad", Match: MatchRule{Level: "error"}},
	})
	if err == nil {
		t.Error("expected error for rule with no notification target")
	}
}
