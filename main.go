package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	erclient "github.com/dmoose/eventrelay/client"
)

// version is set at build time via ldflags.
var version = "dev"

// defaultPort is the default listen port, shared between server and send command.
const defaultPort = 6060

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

	port := flag.Int("port", defaultPort, "listen port")
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

	// Load config — YAML settings provide defaults, flags override
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

	// Apply config file defaults — flags take precedence over YAML
	if cfg != nil && cfg.Server != nil {
		if cfg.Server.Port != 0 && !flagSet("port") {
			*port = cfg.Server.Port
		}
		if cfg.Server.Bind != "" && !flagSet("bind") {
			*bind = cfg.Server.Bind
		}
		if cfg.Server.Token != "" && !flagSet("token") {
			*token = cfg.Server.Token
		}
		if cfg.Server.Buffer != 0 && !flagSet("buffer") {
			*bufSize = cfg.Server.Buffer
		}
		if cfg.Server.Log != "" && !flagSet("log") {
			*logFile = cfg.Server.Log
		}
	}

	hub := NewHub(*bufSize)

	// Log hub — separate buffer for structured logs
	logBufSize := 500
	logMinLevel := "debug"
	if cfg != nil && cfg.Server != nil {
		if cfg.Server.LogBuffer != 0 {
			logBufSize = cfg.Server.LogBuffer
		}
		if cfg.Server.LogMinLevel != "" {
			logMinLevel = cfg.Server.LogMinLevel
		}
	}
	logHub := NewLogHub(logBufSize, logMinLevel)

	var notifier *Notifier
	if cfg != nil && len(cfg.Notify) > 0 {
		var err error
		notifier, err = NewNotifier(cfg.Notify)
		if err != nil {
			log.Fatalf("setting up notifications: %v", err)
		}
		defer notifier.Close()
	}

	var logWriter io.Writer
	if *logFile != "" {
		f, err := os.OpenFile(*logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			log.Fatalf("opening log file: %v", err)
		}
		defer f.Close() //nolint:errcheck // best-effort log file close at shutdown
		logWriter = f
	}

	mux := http.NewServeMux()

	postHandler := postEventHandler(hub, logWriter, notifier)
	batchHandler := batchEventHandler(hub, logWriter, notifier)
	if *token != "" {
		postHandler = requireToken(*token, postHandler)
		batchHandler = requireToken(*token, batchHandler)
	}
	mux.HandleFunc("/events", postHandler)
	mux.HandleFunc("/events/batch", batchHandler)
	mux.HandleFunc("/events/stream", sseStreamHandler(hub))
	mux.HandleFunc("/events/recent", recentHandler(hub))
	mux.HandleFunc("/events/stats", statsHandler(hub))
	mux.HandleFunc("/events/rate", rateHistoryHandler(hub))
	mux.HandleFunc("/events/channels", channelsHandler(hub))
	logPostHandler := postLogHandler(logHub)
	if *token != "" {
		logPostHandler = requireToken(*token, logPostHandler)
	}
	mux.HandleFunc("/log", logPostHandler)
	mux.HandleFunc("/logs/stream", logSSEHandler(logHub))
	mux.HandleFunc("/logs/recent", logRecentHandler(logHub))
	mux.HandleFunc("/logs/stats", logStatsHandler(logHub))
	mux.HandleFunc("/logs/rate", logRateHistoryHandler(logHub))
	mux.HandleFunc("/healthz", healthHandler())

	// Pages system
	startTime := time.Now()
	var pageRunner *PageRunner
	if cfg != nil && len(cfg.Pages) > 0 {
		scriptsDir := ""
		if cfg.Server != nil {
			scriptsDir = cfg.Server.ScriptsDir
		}
		pageRunner = NewPageRunner(cfg.Pages, scriptsDir)
	}
	if pageRunner != nil {
		mux.HandleFunc("/api/pages", pagesListHandler(pageRunner))
		mux.HandleFunc("/api/pages/", pageContentHandler(pageRunner))
	} else {
		// Empty list when no pages configured
		mux.HandleFunc("/api/pages", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("[]"))
		})
	}
	mux.HandleFunc("/api/status", statusPageHandler(hub, logHub, notifier, cfg, startTime))

	staticSub, _ := fs.Sub(staticFS, "static")
	staticHandler := http.FileServer(http.FS(staticSub))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		staticHandler.ServeHTTP(w, r)
	}))

	handler := corsMiddleware(mux)

	addr := fmt.Sprintf("%s:%d", *bind, *port)
	server := &http.Server{Addr: addr, Handler: handler}

	// Graceful shutdown on SIGINT/SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Self-logging: once the server is up, set up slog to tee to stderr + our own /log endpoint
	selfLog := true
	if cfg != nil && cfg.Server != nil && cfg.Server.SelfLog != nil {
		selfLog = *cfg.Server.SelfLog
	}

	go func() {
		<-ctx.Done()
		slog.Info("shutting down")
		if err := server.Shutdown(context.Background()); err != nil {
			slog.Error("shutdown error", "error", err)
		}
	}()

	// Start listening in a goroutine so we can set up self-logging after
	listenErr := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			listenErr <- err
		}
		close(listenErr)
	}()

	// Brief pause to let the listener start, then set up self-logging
	time.Sleep(50 * time.Millisecond)
	select {
	case err := <-listenErr:
		if err != nil {
			log.Fatal(err)
		}
		return
	default:
	}

	if selfLog {
		selfURL := fmt.Sprintf("http://127.0.0.1:%d/events", *port)
		erClient := erclient.New(selfURL, "eventrelay")
		logHandler := erclient.NewSlogLogHandler(erClient, &erclient.SlogLogOptions{
			Logger:    "eventrelay",
			AddSource: true,
		})
		// Tee: write to both stderr (text) and eventrelay /log endpoint
		stderrHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
		slog.SetDefault(slog.New(teeHandler{stderrHandler, logHandler}))
		defer erClient.Flush()
	}

	slog.Info("server started",
		"version", version,
		"addr", addr,
		"pid", os.Getpid(),
		"buffer", *bufSize,
		"log_buffer", logBufSize,
		"log_min_level", logMinLevel,
		"self_log", selfLog,
	)

	if cfg != nil && cfg.Server != nil {
		if len(cfg.Notify) > 0 {
			slog.Info("notifications enabled", "rules", len(cfg.Notify))
		}
		if *logFile != "" {
			slog.Info("event log file", "path", *logFile)
		}
	}

	// Block until server exits
	if err := <-listenErr; err != nil {
		log.Fatal(err)
	}
}

// teeHandler fans out slog records to multiple handlers.
type teeHandler struct {
	a, b slog.Handler
}

func (t teeHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return t.a.Enabled(ctx, l) || t.b.Enabled(ctx, l)
}

func (t teeHandler) Handle(ctx context.Context, r slog.Record) error {
	_ = t.a.Handle(ctx, r)
	_ = t.b.Handle(ctx, r)
	return nil
}

func (t teeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return teeHandler{t.a.WithAttrs(attrs), t.b.WithAttrs(attrs)}
}

func (t teeHandler) WithGroup(name string) slog.Handler {
	return teeHandler{t.a.WithGroup(name), t.b.WithGroup(name)}
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

// flagSet returns true if the named flag was explicitly set on the command line.
func flagSet(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
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
