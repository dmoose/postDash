package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
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

		evt, err := hub.Publish(json.RawMessage(body))
		if err != nil {
			http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Tee to log
		if logWriter != nil {
			line, _ := json.Marshal(evt)
			fmt.Fprintf(logWriter, "%s\n", line)
		}

		// Check notification rules
		if notifier != nil {
			go notifier.Check(*evt)
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

		filter := filterFromQuery(r)
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

		filter := filterFromQuery(r)
		events := hub.Recent(n, filter)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(events)
	}
}

func statsHandler(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(hub.Stats())
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
