package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPostEventRequiresSource(t *testing.T) {
	hub := NewHub(100)
	handler := postEventHandler(hub, nil, nil)

	// Missing source should fail
	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(`{"action":"test"}`))
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing source, got %d", w.Code)
	}

	// Empty source should fail
	req = httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(`{"source":"","action":"test"}`))
	w = httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty source, got %d", w.Code)
	}

	// Valid source should succeed
	req = httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(`{"source":"myapp","action":"test"}`))
	w = httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for valid source, got %d", w.Code)
	}
}

func TestBatchEventHandler(t *testing.T) {
	hub := NewHub(100)
	handler := batchEventHandler(hub, nil, nil)

	body := `[{"source":"a","action":"one"},{"source":"b","action":"two"}]`
	req := httptest.NewRequest(http.MethodPost, "/events/batch", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	events := hub.Recent(10, Filter{})
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
}

func TestBatchRejectsMissingSource(t *testing.T) {
	hub := NewHub(100)
	handler := batchEventHandler(hub, nil, nil)

	body := `[{"source":"ok"},{"action":"no_source"}]`
	req := httptest.NewRequest(http.MethodPost, "/events/batch", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing source in batch, got %d", w.Code)
	}
}

func TestHealthEndpoint(t *testing.T) {
	handler := healthHandler()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"ok":true`) {
		t.Errorf("expected ok:true in body, got %s", w.Body.String())
	}
}

func TestRequireToken(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := requireToken("secret123", inner)

	tests := []struct {
		name   string
		header string
		want   int
	}{
		{"no header", "", http.StatusUnauthorized},
		{"wrong token", "Bearer wrong", http.StatusUnauthorized},
		{"missing Bearer prefix", "secret123", http.StatusUnauthorized},
		{"valid token", "Bearer secret123", http.StatusOK},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodPost, "/events", nil)
		if tt.header != "" {
			req.Header.Set("Authorization", tt.header)
		}
		w := httptest.NewRecorder()
		handler(w, req)
		if w.Code != tt.want {
			t.Errorf("%s: expected %d, got %d", tt.name, tt.want, w.Code)
		}
	}
}

func TestCORSPreflight(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := corsMiddleware(mux)

	// OPTIONS request should return 204 with CORS headers
	req := httptest.NewRequest(http.MethodOptions, "/events", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204 for OPTIONS, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("missing Access-Control-Allow-Origin header")
	}
	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("missing Access-Control-Allow-Methods header")
	}

	// Regular GET should also have CORS headers
	req = httptest.NewRequest(http.MethodGet, "/events", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("missing CORS header on regular request")
	}
}
