package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// runSend handles the "eventrelay send" subcommand.
func runSend(args []string) {
	fs := flag.NewFlagSet("send", flag.ExitOnError)
	source := fs.String("source", "", "event source (required)")
	fs.StringVar(source, "s", "", "event source (shorthand)")
	action := fs.String("action", "", "event action")
	fs.StringVar(action, "a", "", "event action (shorthand)")
	level := fs.String("level", "info", "event level (info, warn, error, debug)")
	fs.StringVar(level, "l", "info", "event level (shorthand)")
	channel := fs.String("channel", "", "event channel")
	fs.StringVar(channel, "c", "", "event channel (shorthand)")
	agentID := fs.String("agent-id", "", "agent identifier")
	data := fs.String("data", "", "JSON data payload")
	fs.StringVar(data, "d", "", "JSON data payload (shorthand)")
	token := fs.String("token", "", "Bearer token for auth")
	fs.StringVar(token, "t", "", "Bearer token (shorthand)")
	port := fs.Int("port", defaultPort, "server port")
	fs.IntVar(port, "p", defaultPort, "server port (shorthand)")
	url := fs.String("url", "", "full server URL (overrides --port)")
	stdin := fs.Bool("stdin", false, "read JSON event from stdin")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: eventrelay send [flags]\n\nSend an event to a running eventrelay server.\n\nFlags:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  eventrelay send -s myapp -a deploy -d '{\"env\":\"prod\"}'\n")
		fmt.Fprintf(os.Stderr, "  eventrelay send --source ci --action build_done --level info\n")
		fmt.Fprintf(os.Stderr, "  echo '{\"source\":\"ci\",\"action\":\"done\"}' | eventrelay send --stdin\n")
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	serverURL := *url
	if serverURL == "" {
		serverURL = fmt.Sprintf("http://localhost:%d", *port)
	}
	endpoint := serverURL + "/events"

	var body []byte

	if *stdin {
		var err error
		body, err = io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading stdin: %v\n", err)
			os.Exit(1)
		}
		// Validate it's valid JSON
		if !json.Valid(body) {
			fmt.Fprintf(os.Stderr, "stdin is not valid JSON\n")
			os.Exit(1)
		}
	} else {
		if *source == "" {
			fmt.Fprintf(os.Stderr, "error: --source (-s) is required\n")
			fs.Usage()
			os.Exit(1)
		}

		evt := map[string]any{
			"source": *source,
			"level":  *level,
		}
		if *action != "" {
			evt["action"] = *action
		}
		if *channel != "" {
			evt["channel"] = *channel
		}
		if *agentID != "" {
			evt["agent_id"] = *agentID
		}
		if *data != "" {
			var d map[string]any
			if err := json.Unmarshal([]byte(*data), &d); err != nil {
				fmt.Fprintf(os.Stderr, "error: --data must be valid JSON: %v\n", err)
				os.Exit(1)
			}
			evt["data"] = d
		}

		var err error
		body, err = json.Marshal(evt)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/json")
	if *token != "" {
		req.Header.Set("Authorization", "Bearer "+*token)
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	respBody, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "server error (%d): %s\n", resp.StatusCode, string(respBody))
		os.Exit(1)
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err == nil {
		if seq, ok := result["seq"]; ok {
			fmt.Fprintf(os.Stderr, "sent (seq %v)\n", seq)
		}
	}
}
