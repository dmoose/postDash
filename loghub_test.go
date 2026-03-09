package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestLogPublishAndRecent(t *testing.T) {
	lh := NewLogHub(100, "debug")

	raw := json.RawMessage(`{"level":"info","message":"hello world","logger":"test"}`)
	entry, accepted, err := lh.Publish(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !accepted {
		t.Fatal("expected accepted")
	}
	if entry.Seq != 1 {
		t.Errorf("expected seq 1, got %d", entry.Seq)
	}
	if entry.Message != "hello world" {
		t.Errorf("expected message 'hello world', got %q", entry.Message)
	}

	entries := lh.Recent(10, "")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Logger != "test" {
		t.Error("logger not preserved")
	}
}

func TestLogLevelGating(t *testing.T) {
	lh := NewLogHub(100, "warn")

	// Debug should be gated
	_, accepted, err := lh.Publish(json.RawMessage(`{"level":"debug","message":"ignored"}`))
	if err != nil {
		t.Fatal(err)
	}
	if accepted {
		t.Fatal("expected debug to be gated when min_level=warn")
	}

	// Info should be gated
	_, accepted, err = lh.Publish(json.RawMessage(`{"level":"info","message":"also ignored"}`))
	if err != nil {
		t.Fatal(err)
	}
	if accepted {
		t.Fatal("expected info to be gated when min_level=warn")
	}

	// Warn should be accepted
	_, accepted, err = lh.Publish(json.RawMessage(`{"level":"warn","message":"warning!"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !accepted {
		t.Fatal("expected warn to be accepted")
	}

	// Error should be accepted
	_, accepted, err = lh.Publish(json.RawMessage(`{"level":"error","message":"bad"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !accepted {
		t.Fatal("expected error to be accepted")
	}

	entries := lh.Recent(10, "")
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (warn+error), got %d", len(entries))
	}

	stats := lh.Stats()
	if stats.Gated != 2 {
		t.Errorf("expected 2 gated, got %d", stats.Gated)
	}
	if stats.Accepted != 2 {
		t.Errorf("expected 2 accepted, got %d", stats.Accepted)
	}
}

func TestLogSubscribeFiltered(t *testing.T) {
	lh := NewLogHub(100, "debug")
	sub := lh.Subscribe("warn")

	_, _, _ = lh.Publish(json.RawMessage(`{"level":"info","message":"skip"}`))
	_, _, _ = lh.Publish(json.RawMessage(`{"level":"error","message":"catch"}`))

	select {
	case entry := <-sub.Ch:
		if entry.Message != "catch" {
			t.Errorf("expected message 'catch', got %q", entry.Message)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for log entry")
	}

	// Should not have received info
	select {
	case entry := <-sub.Ch:
		t.Errorf("unexpected entry: %+v", entry)
	default:
	}

	lh.Unsubscribe(sub)
}

func TestLogBufferOverflow(t *testing.T) {
	lh := NewLogHub(3, "debug")
	for i := range 5 {
		raw := json.RawMessage(`{"level":"info","message":"msg` + string(rune('0'+i)) + `"}`)
		_, _, _ = lh.Publish(raw)
	}

	entries := lh.Recent(10, "")
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Seq != 3 {
		t.Errorf("expected oldest seq 3, got %d", entries[0].Seq)
	}
}

func TestLogStats(t *testing.T) {
	lh := NewLogHub(100, "debug")
	_, _, _ = lh.Publish(json.RawMessage(`{"level":"info","message":"a","logger":"app"}`))
	_, _, _ = lh.Publish(json.RawMessage(`{"level":"error","message":"b","logger":"app"}`))
	_, _, _ = lh.Publish(json.RawMessage(`{"level":"info","message":"c","logger":"db"}`))

	stats := lh.Stats()
	if stats.Accepted != 3 {
		t.Errorf("expected 3 accepted, got %d", stats.Accepted)
	}
	if stats.ByLevel["info"] != 2 {
		t.Errorf("expected 2 info, got %d", stats.ByLevel["info"])
	}
	if stats.ByLevel["error"] != 1 {
		t.Errorf("expected 1 error, got %d", stats.ByLevel["error"])
	}
	if stats.ByLogger["app"] != 2 {
		t.Errorf("expected 2 from app logger, got %d", stats.ByLogger["app"])
	}
	if stats.ByLogger["db"] != 1 {
		t.Errorf("expected 1 from db logger, got %d", stats.ByLogger["db"])
	}
}

func TestLogDefaultLevel(t *testing.T) {
	lh := NewLogHub(100, "debug")
	entry, accepted, _ := lh.Publish(json.RawMessage(`{"message":"no level"}`))
	if !accepted {
		t.Fatal("expected accepted")
	}
	if entry.Level != "info" {
		t.Errorf("expected default level info, got %s", entry.Level)
	}
}

func TestLogFields(t *testing.T) {
	lh := NewLogHub(100, "debug")
	_, _, _ = lh.Publish(json.RawMessage(`{"level":"info","message":"with fields","fields":{"request_id":"abc","status":200}}`))

	entries := lh.Recent(1, "")
	if entries[0].Fields["request_id"] != "abc" {
		t.Error("fields not preserved")
	}
}
