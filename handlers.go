package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

func postEventHandler(hub *Hub, logWriter io.Writer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB max
		if err != nil {
			http.Error(w, "read error", http.StatusBadRequest)
			return
		}

		evt, err := hub.Publish(json.RawMessage(body))
		if err != nil {
			http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Tee to log if configured
		if logWriter != nil {
			line, _ := json.Marshal(evt)
			fmt.Fprintf(logWriter, "%s\n", line)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"seq": evt.Seq})
	}
}

func sseStreamHandler(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		filter := Filter{
			Source:  r.URL.Query().Get("source"),
			Action:  r.URL.Query().Get("action"),
			AgentID: r.URL.Query().Get("agent_id"),
		}

		client := hub.Subscribe(filter)
		defer hub.Unsubscribe(client)

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		flusher.Flush()

		ctx := r.Context()
		for {
			select {
			case evt, ok := <-client.ch:
				if !ok {
					return
				}
				data, _ := json.Marshal(evt)
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			case <-ctx.Done():
				return
			}
		}
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

		filter := Filter{
			Source:  r.URL.Query().Get("source"),
			Action:  r.URL.Query().Get("action"),
			AgentID: r.URL.Query().Get("agent_id"),
		}

		events := hub.Recent(n, filter)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(events)
	}
}
