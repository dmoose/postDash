package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPublishAndRecent(t *testing.T) {
	hub := NewHub(100)

	raw := json.RawMessage(`{"source":"test","action":"hello","data":{"key":"val"}}`)
	evt, err := hub.Publish(raw)
	if err != nil {
		t.Fatal(err)
	}
	if evt.Seq != 1 {
		t.Errorf("expected seq 1, got %d", evt.Seq)
	}
	if evt.Source != "test" {
		t.Errorf("expected source test, got %s", evt.Source)
	}
	if evt.Level != "info" {
		t.Errorf("expected default level info, got %s", evt.Level)
	}

	events := hub.Recent(10, Filter{})
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Data["key"] != "val" {
		t.Error("data not preserved")
	}
}

func TestMatchesEventSharedHelper(t *testing.T) {
	evt := Event{
		Source:  "app",
		Channel: "ops",
		Action:  "deploy",
		Level:   "error",
		AgentID: "agent-1",
	}
	if !matchesEvent("app", "ops", "deploy", "error", "agent-1", evt) {
		t.Fatal("expected full match to succeed")
	}
	if matchesEvent("app", "ops", "deploy", "info", "agent-1", evt) {
		t.Fatal("expected mismatched level to fail")
	}
}

func TestBufferUsage(t *testing.T) {
	hub := NewHub(3)
	used, cap := hub.BufferUsage()
	if used != 0 || cap != 3 {
		t.Fatalf("expected initial usage 0/3, got %d/%d", used, cap)
	}
	_, _ = hub.Publish(json.RawMessage(`{"source":"a"}`))
	_, _ = hub.Publish(json.RawMessage(`{"source":"b"}`))
	used, cap = hub.BufferUsage()
	if used != 2 || cap != 3 {
		t.Fatalf("expected usage 2/3, got %d/%d", used, cap)
	}
}

func TestRingBufferOverflow(t *testing.T) {
	hub := NewHub(3)
	for i := range 5 {
		raw := json.RawMessage(`{"source":"test","action":"` + string(rune('a'+i)) + `"}`)
		_, _ = hub.Publish(raw)
	}

	events := hub.Recent(10, Filter{})
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	if events[0].Seq != 3 {
		t.Errorf("expected oldest seq 3, got %d", events[0].Seq)
	}
}

func TestFilterMatching(t *testing.T) {
	hub := NewHub(100)
	_, _ = hub.Publish(json.RawMessage(`{"source":"a","channel":"ch1","action":"x"}`))
	_, _ = hub.Publish(json.RawMessage(`{"source":"b","channel":"ch2","action":"y"}`))
	_, _ = hub.Publish(json.RawMessage(`{"source":"a","channel":"ch1","action":"z","level":"error"}`))

	tests := []struct {
		filter   Filter
		expected int
	}{
		{Filter{}, 3},
		{Filter{Source: "a"}, 2},
		{Filter{Channel: "ch1"}, 2},
		{Filter{Level: "error"}, 1},
		{Filter{Source: "a", Level: "error"}, 1},
		{Filter{Source: "nonexistent"}, 0},
	}

	for _, tt := range tests {
		events := hub.Recent(100, tt.filter)
		if len(events) != tt.expected {
			t.Errorf("filter %+v: expected %d, got %d", tt.filter, tt.expected, len(events))
		}
	}
}

func TestSubscribeReceivesEvents(t *testing.T) {
	hub := NewHub(100)
	client := hub.Subscribe(Filter{Source: "target"})

	_, _ = hub.Publish(json.RawMessage(`{"source":"other","action":"ignore"}`))
	_, _ = hub.Publish(json.RawMessage(`{"source":"target","action":"catch"}`))

	select {
	case evt := <-client.Ch:
		if evt.Action != "catch" {
			t.Errorf("expected action catch, got %s", evt.Action)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Should not have received the "other" event
	select {
	case evt := <-client.Ch:
		t.Errorf("unexpected event: %+v", evt)
	default:
	}

	hub.Unsubscribe(client)
}

func TestTimestampAutoSet(t *testing.T) {
	hub := NewHub(100)
	before := time.Now()
	_, _ = hub.Publish(json.RawMessage(`{"source":"test"}`))
	after := time.Now()

	events := hub.Recent(1, Filter{})
	if events[0].TS.Before(before) || events[0].TS.After(after) {
		t.Error("auto-set timestamp out of range")
	}
}

func TestStats(t *testing.T) {
	hub := NewHub(100)
	_, _ = hub.Publish(json.RawMessage(`{"source":"a","level":"info","channel":"ops"}`))
	_, _ = hub.Publish(json.RawMessage(`{"source":"a","level":"error","channel":"ops"}`))
	_, _ = hub.Publish(json.RawMessage(`{"source":"b","level":"info","channel":"deploy"}`))

	stats := hub.Stats()
	if stats.TotalEvents != 3 {
		t.Errorf("expected 3 total events, got %d", stats.TotalEvents)
	}
	if stats.BySource["a"] != 2 {
		t.Errorf("expected 2 events from source a, got %d", stats.BySource["a"])
	}
	if stats.BySource["b"] != 1 {
		t.Errorf("expected 1 event from source b, got %d", stats.BySource["b"])
	}
	if stats.ByLevel["info"] != 2 {
		t.Errorf("expected 2 info events, got %d", stats.ByLevel["info"])
	}
	if stats.ByLevel["error"] != 1 {
		t.Errorf("expected 1 error event, got %d", stats.ByLevel["error"])
	}
	if stats.ByChannel["ops"] != 2 {
		t.Errorf("expected 2 ops events, got %d", stats.ByChannel["ops"])
	}
	if stats.ByChannel["deploy"] != 1 {
		t.Errorf("expected 1 deploy event, got %d", stats.ByChannel["deploy"])
	}
	// Events were just published, rate should be > 0
	if stats.RecentRate <= 0 {
		t.Error("expected non-zero recent rate for just-published events")
	}
}

func TestRateHistory(t *testing.T) {
	hub := NewHub(100)
	// Publish a few events (all with "now" timestamps)
	for range 5 {
		_, _ = hub.Publish(json.RawMessage(`{"source":"test"}`))
	}

	counts := hub.RateHistory(5*time.Minute, 10)
	if len(counts) != 10 {
		t.Fatalf("expected 10 buckets, got %d", len(counts))
	}

	// All events are recent, so they should land in the last bucket
	total := 0
	for _, c := range counts {
		total += c
	}
	if total != 5 {
		t.Errorf("expected 5 total events across buckets, got %d", total)
	}
	// Last bucket should have all the events (they were all just now)
	if counts[9] != 5 {
		t.Errorf("expected 5 events in last bucket, got %d", counts[9])
	}
}

func TestChannels(t *testing.T) {
	hub := NewHub(100)
	_, _ = hub.Publish(json.RawMessage(`{"source":"a","channel":"alpha"}`))
	_, _ = hub.Publish(json.RawMessage(`{"source":"b","channel":"beta"}`))
	_, _ = hub.Publish(json.RawMessage(`{"source":"c"}`)) // no channel

	channels := hub.Channels()
	if len(channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(channels))
	}

	found := map[string]bool{}
	for _, ch := range channels {
		found[ch] = true
	}
	if !found["alpha"] || !found["beta"] {
		t.Errorf("expected alpha and beta channels, got %v", channels)
	}
}

func TestEnrichedFields(t *testing.T) {
	hub := NewHub(100)
	_, _ = hub.Publish(json.RawMessage(`{"source":"app","channel":"ops","level":"warn","duration_ms":42}`))

	events := hub.Recent(1, Filter{})
	evt := events[0]
	if evt.Channel != "ops" {
		t.Errorf("expected channel ops, got %s", evt.Channel)
	}
	if evt.Level != "warn" {
		t.Errorf("expected level warn, got %s", evt.Level)
	}
	if evt.DurationMS == nil || *evt.DurationMS != 42 {
		t.Errorf("expected duration_ms 42, got %v", evt.DurationMS)
	}
}
