package main

import (
	"embed"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

//go:embed static
var staticFS embed.FS

func main() {
	port := flag.Int("port", 6060, "listen port")
	bind := flag.String("bind", "127.0.0.1", "bind address (use 0.0.0.0 for network access)")
	token := flag.String("token", "", "require Bearer token for POST /events")
	logFile := flag.String("log", "", "optional JSONL log file path")
	bufSize := flag.Int("buffer", 1000, "ring buffer size")
	configFile := flag.String("config", "", "config file path (eventrelay.yaml)")
	tuiMode := flag.Bool("tui", false, "connect to a running eventrelay as a TUI dashboard")
	tuiURL := flag.String("url", "", "eventrelay server URL for TUI mode (default http://localhost:<port>)")
	flag.Parse()

	// TUI client mode — connect to an existing server
	if *tuiMode {
		url := *tuiURL
		if url == "" {
			url = fmt.Sprintf("http://localhost:%d", *port)
		}
		p := tea.NewProgram(newTUIModel(url), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			log.Fatal(err)
		}
		return
	}

	// Load config if provided
	var cfg *Config
	if *configFile != "" {
		var err error
		cfg, err = LoadConfig(*configFile)
		if err != nil {
			log.Fatalf("loading config: %v", err)
		}
		log.Printf("Loaded config from %s (%d notification rules)", *configFile, len(cfg.Notify))
	}

	hub := NewHub(*bufSize)

	// Set up notifier if config has rules
	var notifier *Notifier
	if cfg != nil && len(cfg.Notify) > 0 {
		var err error
		notifier, err = NewNotifier(cfg.Notify)
		if err != nil {
			log.Fatalf("setting up notifications: %v", err)
		}
		log.Printf("Notifications enabled: %d rules", len(cfg.Notify))
	}

	var logWriter io.Writer
	if *logFile != "" {
		f, err := os.OpenFile(*logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("opening log file: %v", err)
		}
		defer f.Close()
		logWriter = f
		log.Printf("Logging events to %s", *logFile)
	}

	mux := http.NewServeMux()

	// POST handler with optional auth and notification
	postHandler := postEventHandler(hub, logWriter, notifier)
	if *token != "" {
		postHandler = requireToken(*token, postHandler)
		log.Printf("Token auth enabled for POST /events")
	}
	mux.HandleFunc("/events", postHandler)
	mux.HandleFunc("/events/stream", sseStreamHandler(hub))
	mux.HandleFunc("/events/recent", recentHandler(hub))
	mux.HandleFunc("/events/stats", statsHandler(hub))

	staticSub, _ := fs.Sub(staticFS, "static")
	mux.Handle("/", http.FileServer(http.FS(staticSub)))

	addr := fmt.Sprintf("%s:%d", *bind, *port)
	log.Printf("eventrelay listening on http://%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

// requireToken wraps a handler to require a Bearer token.
func requireToken(token string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}
