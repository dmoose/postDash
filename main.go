package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
)

// version is set at build time via ldflags.
var version = "dev"

//go:embed static
var staticFS embed.FS

func main() {
	// Subcommand dispatch — check before flag parsing
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "send":
			runSend(os.Args[2:])
			return
		case "version":
			fmt.Println("eventrelay " + version)
			return
		}
	}

	port := flag.Int("port", 6060, "listen port")
	bind := flag.String("bind", "127.0.0.1", "bind address (use 0.0.0.0 for network access)")
	token := flag.String("token", "", "require Bearer token for POST /events")
	logFile := flag.String("log", "", "optional JSONL log file path")
	bufSize := flag.Int("buffer", 1000, "ring buffer size")
	configFile := flag.String("config", defaultConfigPath(), "config file path")
	tuiMode := flag.Bool("tui", false, "connect to a running eventrelay as a TUI dashboard")
	tuiURL := flag.String("url", "", "eventrelay server URL for TUI mode")
	statusMode := flag.Bool("status", false, "check if eventrelay is running")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("eventrelay " + version)
		return
	}

	// Status check mode
	if *statusMode {
		runStatus(*port)
		return
	}

	// TUI client mode
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

	// Server mode — check for existing instance
	pidPath := DefaultPIDPath()
	if pid, running, _ := ReadPIDFile(pidPath); running {
		log.Fatalf("eventrelay already running (pid %d). Use --status to check, or remove %s", pid, pidPath)
	}
	if CheckPort(*port) {
		log.Fatalf("port %d already in use", *port)
	}

	pidFile, err := WritePIDFile(pidPath)
	if err != nil {
		log.Fatal(err)
	}
	defer pidFile.Remove()

	// Load config
	var cfg *Config
	if *configFile != "" {
		if _, err := os.Stat(*configFile); err == nil {
			cfg, err = LoadConfig(*configFile)
			if err != nil {
				log.Fatalf("loading config: %v", err) //nolint:gocritic // fatal is intentional; pidFile cleanup is best-effort
			}
			log.Printf("Loaded config from %s (%d notification rules)", *configFile, len(cfg.Notify))
		}
	}

	hub := NewHub(*bufSize)

	var notifier *Notifier
	if cfg != nil && len(cfg.Notify) > 0 {
		var err error
		notifier, err = NewNotifier(cfg.Notify)
		if err != nil {
			log.Fatalf("setting up notifications: %v", err)
		}
		log.Printf("Notifications enabled: %d rules", len(cfg.Notify))
		defer notifier.Close()
	}

	var logWriter io.Writer
	if *logFile != "" {
		f, err := os.OpenFile(*logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("opening log file: %v", err)
		}
		defer f.Close() //nolint:errcheck // best-effort log file close at shutdown
		logWriter = f
		log.Printf("Logging events to %s", *logFile)
	}

	mux := http.NewServeMux()

	postHandler := postEventHandler(hub, logWriter, notifier)
	batchHandler := batchEventHandler(hub, logWriter, notifier)
	if *token != "" {
		postHandler = requireToken(*token, postHandler)
		batchHandler = requireToken(*token, batchHandler)
		log.Printf("Token auth enabled for POST /events")
	}
	mux.HandleFunc("/events", postHandler)
	mux.HandleFunc("/events/batch", batchHandler)
	mux.HandleFunc("/events/stream", sseStreamHandler(hub))
	mux.HandleFunc("/events/recent", recentHandler(hub))
	mux.HandleFunc("/events/stats", statsHandler(hub))
	mux.HandleFunc("/events/rate", rateHistoryHandler(hub))
	mux.HandleFunc("/events/channels", channelsHandler(hub))
	mux.HandleFunc("/healthz", healthHandler())

	staticSub, _ := fs.Sub(staticFS, "static")
	mux.Handle("/", http.FileServer(http.FS(staticSub)))

	handler := corsMiddleware(mux)

	addr := fmt.Sprintf("%s:%d", *bind, *port)
	server := &http.Server{Addr: addr, Handler: handler}

	// Graceful shutdown on SIGINT/SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		log.Println("shutting down...")
		if err := server.Shutdown(context.Background()); err != nil {
			log.Printf("shutdown error: %v", err)
		}
	}()

	log.Printf("eventrelay %s listening on http://%s (pid %d)", version, addr, os.Getpid())
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func runStatus(port int) {
	pidPath := DefaultPIDPath()
	pid, running, _ := ReadPIDFile(pidPath)

	if running {
		fmt.Printf("eventrelay is running (pid %d)\n", pid)
		if CheckPort(port) {
			fmt.Printf("  listening on port %d\n", port)
			fmt.Printf("  dashboard: http://localhost:%d\n", port)
		}
	} else {
		fmt.Println("eventrelay is not running")
		if CheckPort(port) {
			fmt.Printf("  (but port %d is in use by another process)\n", port)
		}
	}
}

func defaultConfigPath() string {
	if dir := os.Getenv("EVENTRELAY_CONFIG_DIR"); dir != "" {
		return dir + "/eventrelay.yaml"
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		return home + "/.config/eventrelay/eventrelay.yaml"
	}
	return ""
}

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
