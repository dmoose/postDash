package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

// PageConf defines a command-backed page in the dashboard.
type PageConf struct {
	Name     string `yaml:"name"`               // display name
	Command  string `yaml:"command"`            // shell command to execute
	Format   string `yaml:"format,omitempty"`   // text, json, yaml, markdown (default: text)
	Interval string `yaml:"interval,omitempty"` // cache duration, e.g. "30s", "5m" (default: 10s)
}

// pageResult is the cached output of a page command.
type pageResult struct {
	Content   string    `json:"content"`
	Format    string    `json:"format"`
	UpdatedAt time.Time `json:"updated_at"`
	Error     string    `json:"error,omitempty"`
}

// PageRunner manages page command execution and caching.
type PageRunner struct {
	pages      []PageConf
	scriptsDir string
	cache      map[string]*pageResult
	mu         sync.RWMutex
	interval   map[string]time.Duration
}

// NewPageRunner creates a PageRunner from config.
func NewPageRunner(pages []PageConf, scriptsDir string) *PageRunner {
	pr := &PageRunner{
		pages:      pages,
		scriptsDir: scriptsDir,
		cache:      make(map[string]*pageResult),
		interval:   make(map[string]time.Duration),
	}
	for _, p := range pages {
		d := 10 * time.Second
		if p.Interval != "" {
			if parsed, err := time.ParseDuration(p.Interval); err == nil {
				d = parsed
			}
		}
		pr.interval[pageSlug(p.Name)] = d
	}
	return pr
}

func (pr *PageRunner) findPage(name string) *PageConf {
	for i := range pr.pages {
		if pageSlug(pr.pages[i].Name) == name {
			return &pr.pages[i]
		}
	}
	return nil
}

// Execute runs a page command and returns the result, using cache if fresh.
func (pr *PageRunner) Execute(name string) *pageResult {
	pr.mu.RLock()
	cached, ok := pr.cache[name]
	interval := pr.interval[name]
	pr.mu.RUnlock()

	if ok && time.Since(cached.UpdatedAt) < interval {
		return cached
	}

	page := pr.findPage(name)
	if page == nil {
		return &pageResult{Error: "page not found", Format: "text", UpdatedAt: time.Now()}
	}

	format := page.Format
	if format == "" {
		format = "text"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", page.Command)
	// Inherit environment and prepend scripts dir to PATH
	cmd.Env = os.Environ()
	if pr.scriptsDir != "" {
		for i, env := range cmd.Env {
			if after, ok0 := strings.CutPrefix(env, "PATH="); ok0 {
				cmd.Env[i] = "PATH=" + pr.scriptsDir + ":" + after
				break
			}
		}
	}
	out, err := cmd.CombinedOutput()

	result := &pageResult{
		Content:   string(out),
		Format:    format,
		UpdatedAt: time.Now(),
	}
	if err != nil {
		result.Error = err.Error()
		if len(out) == 0 {
			result.Content = err.Error()
		}
	}

	pr.mu.Lock()
	pr.cache[name] = result
	pr.mu.Unlock()

	return result
}

// pageSlug converts a page name to a URL-safe slug.
func pageSlug(name string) string {
	s := strings.ToLower(name)
	s = strings.ReplaceAll(s, " ", "-")
	return s
}

// pagesListHandler returns the list of registered pages.
func pagesListHandler(runner *PageRunner) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		type pageInfo struct {
			Name   string `json:"name"`
			Slug   string `json:"slug"`
			Format string `json:"format"`
		}
		pages := make([]pageInfo, 0, len(runner.pages))
		for _, p := range runner.pages {
			f := p.Format
			if f == "" {
				f = "text"
			}
			pages = append(pages, pageInfo{Name: p.Name, Slug: pageSlug(p.Name), Format: f})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(pages)
	}
}

// pageContentHandler executes a page command and returns the result.
func pageContentHandler(runner *PageRunner) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract slug from path: /api/pages/{slug}
		slug := strings.TrimPrefix(r.URL.Path, "/api/pages/")
		if slug == "" {
			http.Error(w, "page slug required", http.StatusBadRequest)
			return
		}

		page := runner.findPage(slug)
		if page == nil {
			http.Error(w, "page not found", http.StatusNotFound)
			return
		}

		result := runner.Execute(slug)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	}
}

// statusSection is a labeled group of key-value pairs for the status page.
type statusSection struct {
	Title string     `json:"title"`
	Items [][]string `json:"items"` // each item is [key, value]
}

// statusPageHandler returns the built-in eventrelay status page.
func statusPageHandler(hub *Hub, notifier *Notifier, cfg *Config, startTime time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats := hub.Stats()
		hostname, _ := os.Hostname()

		server := statusSection{
			Title: "Server",
			Items: [][]string{
				{"Version", version},
				{"Hostname", hostname},
				{"PID", fmt.Sprintf("%d", os.Getpid())},
				{"Uptime", time.Since(startTime).Truncate(time.Second).String()},
				{"Started", startTime.Format("2006-01-02 15:04:05")},
				{"Go", runtime.Version()},
				{"OS / Arch", runtime.GOOS + " / " + runtime.GOARCH},
			},
		}

		bufferUsed, bufferMax := hub.BufferUsage()
		events := statusSection{
			Title: "Events",
			Items: [][]string{
				{"Total Events", fmt.Sprintf("%d", stats.TotalEvents)},
				{"Rate", fmt.Sprintf("%.1f/s", stats.RecentRate)},
				{"Buffer", fmt.Sprintf("%d / %d", bufferUsed, bufferMax)},
				{"SSE Clients", fmt.Sprintf("%d", stats.ClientCount)},
			},
		}

		if len(stats.BySource) > 0 {
			for src, cnt := range stats.BySource {
				events.Items = append(events.Items, []string{"Source: " + src, fmt.Sprintf("%d", cnt)})
			}
		}
		if len(stats.ByLevel) > 0 {
			for lvl, cnt := range stats.ByLevel {
				events.Items = append(events.Items, []string{"Level: " + lvl, fmt.Sprintf("%d", cnt)})
			}
		}

		sections := []statusSection{server, events}

		if cfg != nil {
			config := statusSection{Title: "Configuration"}
			if cfg.Server != nil {
				if cfg.Server.Port != 0 {
					config.Items = append(config.Items, []string{"Port", fmt.Sprintf("%d", cfg.Server.Port)})
				}
				if cfg.Server.Bind != "" {
					config.Items = append(config.Items, []string{"Bind", cfg.Server.Bind})
				}
			}
			config.Items = append(config.Items, []string{"Notify Rules", fmt.Sprintf("%d", len(cfg.Notify))})
			for _, rule := range cfg.Notify {
				config.Items = append(config.Items, []string{"  " + rule.Name, matchRuleDesc(rule.Match)})
			}
			config.Items = append(config.Items, []string{"Pages", fmt.Sprintf("%d", len(cfg.Pages))})
			for _, p := range cfg.Pages {
				config.Items = append(config.Items, []string{"  " + p.Name, p.Command})
			}
			sections = append(sections, config)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sections)
	}
}

func matchRuleDesc(m MatchRule) string {
	var parts []string
	if m.Source != "" {
		parts = append(parts, "source="+m.Source)
	}
	if m.Channel != "" {
		parts = append(parts, "channel="+m.Channel)
	}
	if m.Action != "" {
		parts = append(parts, "action="+m.Action)
	}
	if m.Level != "" {
		parts = append(parts, "level="+m.Level)
	}
	if m.AgentID != "" {
		parts = append(parts, "agent_id="+m.AgentID)
	}
	if len(parts) == 0 {
		return "all events"
	}
	return strings.Join(parts, ", ")
}
