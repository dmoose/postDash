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
)

//go:embed static
var staticFS embed.FS

func main() {
	port := flag.Int("port", 6060, "listen port")
	logFile := flag.String("log", "", "optional JSONL log file path")
	bufSize := flag.Int("buffer", 1000, "ring buffer size")
	flag.Parse()

	hub := NewHub(*bufSize)

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
	mux.HandleFunc("/events", postEventHandler(hub, logWriter))
	mux.HandleFunc("/events/stream", sseStreamHandler(hub))
	mux.HandleFunc("/events/recent", recentHandler(hub))

	// Serve embedded static files at root
	staticSub, _ := fs.Sub(staticFS, "static")
	mux.Handle("/", http.FileServer(http.FS(staticSub)))

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("eventrelay listening on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
