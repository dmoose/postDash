package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

func postEventHandler(hub *Hub, logWriter io.Writer, notifier *Notifier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, "read error", http.StatusBadRequest)
			return
		}

		if eventSourceMissing(body) {
			http.Error(w, "source is required", http.StatusBadRequest)
			return
		}

		evt, err := hub.Publish(json.RawMessage(body))
		if err != nil {
			http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Tee to log
		if logWriter != nil {
			line, _ := json.Marshal(evt)
			_, _ = fmt.Fprintf(logWriter, "%s\n", line)
		}

		// Check notification rules
		if notifier != nil {
			go notifier.Check(*evt)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"seq": evt.Seq})
	}
}

func batchEventHandler(hub *Hub, logWriter io.Writer, notifier *Notifier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20)) // 10MB limit for batches
		if err != nil {
			http.Error(w, "read error", http.StatusBadRequest)
			return
		}

		var rawEvents []json.RawMessage
		if err := json.Unmarshal(body, &rawEvents); err != nil {
			http.Error(w, "expected JSON array: "+err.Error(), http.StatusBadRequest)
			return
		}

		seqs := make([]uint64, 0, len(rawEvents))
		for i, raw := range rawEvents {
			// Sequential non-atomic processing: prior valid events remain accepted
			// if a later event in the same batch is invalid.
			if eventSourceMissing(raw) {
				http.Error(w, fmt.Sprintf("event %d: source is required", i), http.StatusBadRequest)
				return
			}

			evt, err := hub.Publish(raw)
			if err != nil {
				http.Error(w, fmt.Sprintf("event %d: %v", i, err), http.StatusBadRequest)
				return
			}

			if logWriter != nil {
				line, _ := json.Marshal(evt)
				_, _ = fmt.Fprintf(logWriter, "%s\n", line)
			}
			if notifier != nil {
				go notifier.Check(*evt)
			}

			seqs = append(seqs, evt.Seq)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"count": len(seqs), "seqs": seqs})
	}
}

func sseStreamHandler(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		filter := filterFromQuery(r)
		client := hub.Subscribe(filter)
		defer hub.Unsubscribe(client)

		sseLoop(w, r, client.Ch, flusher)
	}
}

func recentHandler(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n := 100
		if s := r.URL.Query().Get("n"); s != "" {
			if v, err := strconv.Atoi(s); err == nil && v > 0 {
				n = v
			}
		}

		filter := filterFromQuery(r)
		events := hub.Recent(n, filter)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(events)
	}
}

func rateHistoryHandler(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Default: 5 minutes, 60 buckets (5s each)
		buckets := 60
		if s := r.URL.Query().Get("buckets"); s != "" {
			if v, err := strconv.Atoi(s); err == nil && v > 0 && v <= 300 {
				buckets = v
			}
		}
		minutes := 5
		if s := r.URL.Query().Get("minutes"); s != "" {
			if v, err := strconv.Atoi(s); err == nil && v > 0 {
				minutes = v
			}
		}
		counts := hub.RateHistory(time.Duration(minutes)*time.Minute, buckets)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(counts)
	}
}

func channelsHandler(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(hub.Channels())
	}
}

func statsHandler(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(hub.Stats())
	}
}

func healthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "version": version})
	}
}

func filterFromQuery(r *http.Request) Filter {
	return Filter{
		Source:  r.URL.Query().Get("source"),
		Channel: r.URL.Query().Get("channel"),
		Action:  r.URL.Query().Get("action"),
		Level:   r.URL.Query().Get("level"),
		AgentID: r.URL.Query().Get("agent_id"),
	}
}

// --- Log handlers ---

func postLogHandler(logHub *LogHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, "read error", http.StatusBadRequest)
			return
		}

		entry, accepted, err := logHub.Publish(json.RawMessage(body))
		if err != nil {
			http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"accepted": accepted,
			"seq":      entry.Seq,
		})
	}
}

func logSSEHandler(logHub *LogHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		level := r.URL.Query().Get("level")
		sub := logHub.Subscribe(level)
		defer logHub.Unsubscribe(sub)

		sseLoop(w, r, sub.Ch, flusher)
	}
}

func logRecentHandler(logHub *LogHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n := 100
		if s := r.URL.Query().Get("n"); s != "" {
			if v, err := strconv.Atoi(s); err == nil && v > 0 {
				n = v
			}
		}

		level := r.URL.Query().Get("level")
		entries := logHub.Recent(n, level)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(entries)
	}
}

func logRateHistoryHandler(logHub *LogHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		buckets := 60
		if s := r.URL.Query().Get("buckets"); s != "" {
			if v, err := strconv.Atoi(s); err == nil && v > 0 && v <= 300 {
				buckets = v
			}
		}
		minutes := 5
		if s := r.URL.Query().Get("minutes"); s != "" {
			if v, err := strconv.Atoi(s); err == nil && v > 0 {
				minutes = v
			}
		}
		counts := logHub.RateHistory(time.Duration(minutes)*time.Minute, buckets)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(counts)
	}
}

func logStatsHandler(logHub *LogHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(logHub.Stats())
	}
}

// sseLoop writes items from a channel as SSE events until the client disconnects.
func sseLoop[T any](w http.ResponseWriter, r *http.Request, ch <-chan T, flusher http.Flusher) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case item, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(item)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-ctx.Done():
			return
		}
	}
}

// corsMiddleware handles CORS preflight requests and sets headers on all responses.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
