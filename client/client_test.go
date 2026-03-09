package client

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestClientEmit(t *testing.T) {
	var mu sync.Mutex
	var received []map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var evt map[string]any
		_ = json.Unmarshal(body, &evt)
		mu.Lock()
		received = append(received, evt)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := New(server.URL, "test-app")
	c.Emit("deploy", map[string]any{"env": "prod"})
	c.Flush()

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 {
		t.Fatalf("expected 1 event, got %d", len(received))
	}
	if received[0]["source"] != "test-app" {
		t.Errorf("expected source test-app, got %v", received[0]["source"])
	}
	if received[0]["action"] != "deploy" {
		t.Errorf("expected action deploy, got %v", received[0]["action"])
	}
	data := received[0]["data"].(map[string]any)
	if data["env"] != "prod" {
		t.Errorf("expected env prod, got %v", data["env"])
	}
}

func TestClientNoOp(t *testing.T) {
	c := New("", "noop")
	c.Emit("anything", nil) // should not panic
	c.Flush()
}

func TestClientTimed(t *testing.T) {
	var mu sync.Mutex
	var received []map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var evt map[string]any
		_ = json.Unmarshal(body, &evt)
		mu.Lock()
		received = append(received, evt)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := New(server.URL, "test")
	done := c.Timed("query", map[string]any{"table": "users"})
	done(map[string]any{"rows": 5})
	c.Flush()

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 {
		t.Fatalf("expected 1 event, got %d", len(received))
	}
	if received[0]["duration_ms"] == nil {
		t.Error("expected duration_ms to be set")
	}
	data := received[0]["data"].(map[string]any)
	if data["table"] != "users" || data["rows"] != float64(5) {
		t.Errorf("data not merged: %v", data)
	}
}

func TestSlogHandler(t *testing.T) {
	var mu sync.Mutex
	var received []map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var evt map[string]any
		_ = json.Unmarshal(body, &evt)
		mu.Lock()
		received = append(received, evt)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := New(server.URL, "myapp")
	handler := NewSlogHandler(c, "logs")
	logger := slog.New(handler)

	logger.Info("request handled", "path", "/api/users", "status", 200)
	logger.Error("db connection failed", "host", "db.local")
	c.Flush()

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 2 {
		t.Fatalf("expected 2 events, got %d", len(received))
	}

	// Find events by action (order is non-deterministic due to goroutines)
	byAction := make(map[string]map[string]any)
	for _, evt := range received {
		byAction[evt["action"].(string)] = evt
	}

	infoEvt := byAction["request handled"]
	if infoEvt == nil {
		t.Fatal("missing 'request handled' event")
	}
	if infoEvt["level"] != "info" {
		t.Errorf("expected level info, got %v", infoEvt["level"])
	}
	if infoEvt["channel"] != "logs" {
		t.Errorf("expected channel logs, got %v", infoEvt["channel"])
	}

	errorEvt := byAction["db connection failed"]
	if errorEvt == nil {
		t.Fatal("missing 'db connection failed' event")
	}
	if errorEvt["level"] != "error" {
		t.Errorf("expected level error, got %v", errorEvt["level"])
	}
}

func TestClientLog(t *testing.T) {
	var mu sync.Mutex
	var received []map[string]any
	var lastPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var entry map[string]any
		_ = json.Unmarshal(body, &entry)
		mu.Lock()
		received = append(received, entry)
		lastPath = r.URL.Path
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := New(server.URL+"/events", "myapp")
	c.LogInfo("request handled", map[string]any{"path": "/api"})
	c.Flush()

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(received))
	}
	if lastPath != "/log" {
		t.Errorf("expected path /log, got %s", lastPath)
	}
	if received[0]["message"] != "request handled" {
		t.Errorf("expected message 'request handled', got %v", received[0]["message"])
	}
	if received[0]["logger"] != "myapp" {
		t.Errorf("expected logger myapp, got %v", received[0]["logger"])
	}
	if received[0]["level"] != "info" {
		t.Errorf("expected level info, got %v", received[0]["level"])
	}
	fields := received[0]["fields"].(map[string]any)
	if fields["path"] != "/api" {
		t.Errorf("expected field path=/api, got %v", fields["path"])
	}
}

func TestSlogLogHandler(t *testing.T) {
	var mu sync.Mutex
	var received []map[string]any
	var paths []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var entry map[string]any
		_ = json.Unmarshal(body, &entry)
		mu.Lock()
		received = append(received, entry)
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := New(server.URL+"/events", "myapp")
	handler := NewSlogLogHandler(c, nil)
	logger := slog.New(handler)

	logger.Info("server started", "port", 6060)
	logger.Error("connection failed", "host", "db.local")
	c.Flush()

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(received))
	}

	// All should go to /log
	for _, p := range paths {
		if p != "/log" {
			t.Errorf("expected path /log, got %s", p)
		}
	}

	// Find by message
	byMsg := make(map[string]map[string]any)
	for _, e := range received {
		byMsg[e["message"].(string)] = e
	}

	info := byMsg["server started"]
	if info == nil {
		t.Fatal("missing 'server started' entry")
	}
	if info["level"] != "info" {
		t.Errorf("expected level info, got %v", info["level"])
	}
	if info["logger"] != "myapp" {
		t.Errorf("expected logger myapp, got %v", info["logger"])
	}
	fields := info["fields"].(map[string]any)
	if fields["port"] != float64(6060) {
		t.Errorf("expected port 6060, got %v", fields["port"])
	}

	errEntry := byMsg["connection failed"]
	if errEntry == nil {
		t.Fatal("missing 'connection failed' entry")
	}
	if errEntry["level"] != "error" {
		t.Errorf("expected level error, got %v", errEntry["level"])
	}
}
