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
	case evt := <-client.ch:
		if evt.Action != "catch" {
			t.Errorf("expected action catch, got %s", evt.Action)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Should not have received the "other" event
	select {
	case evt := <-client.ch:
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
