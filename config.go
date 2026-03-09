package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the eventrelay configuration file.
type Config struct {
	Server *ServerConf  `yaml:"server,omitempty"`
	Pages  []PageConf   `yaml:"pages,omitempty"`
	Notify []NotifyRule `yaml:"notify"`
}

// ServerConf holds server settings that can be set in the YAML config file.
// Flags take precedence over these values.
type ServerConf struct {
	Port        int    `yaml:"port,omitempty"`          // listen port
	Bind        string `yaml:"bind,omitempty"`          // bind address
	Token       string `yaml:"token,omitempty"`         // Bearer token for POST auth
	Buffer      int    `yaml:"buffer,omitempty"`        // ring buffer size
	LogBuffer   int    `yaml:"log_buffer,omitempty"`    // ring buffer size for logs (default: 500)
	LogMinLevel string `yaml:"log_min_level,omitempty"` // minimum log level to accept: debug|info|warn|error (default: debug)
	SelfLog     *bool  `yaml:"self_log,omitempty"`      // log eventrelay internals to its own /log endpoint (default: true)
	Log         string `yaml:"log,omitempty"`           // JSONL log file path
	ScriptsDir  string `yaml:"scripts_dir,omitempty"`   // directory for page scripts
}

// NotifyRule defines when and where to send notifications.
type NotifyRule struct {
	Name     string        `yaml:"name"`               // human label for this rule
	Match    MatchRule     `yaml:"match"`              // event matching criteria
	Webhook  *Webhook      `yaml:"webhook,omitempty"`  // outbound webhook
	Slack    *SlackConf    `yaml:"slack,omitempty"`    // Slack webhook
	Discord  *DiscordConf  `yaml:"discord,omitempty"`  // Discord webhook
	Database *DatabaseConf `yaml:"database,omitempty"` // database storage
}

// MatchRule defines which events trigger a notification.
// All non-empty fields must match (AND logic).
type MatchRule struct {
	Source  string `yaml:"source,omitempty"`
	Channel string `yaml:"channel,omitempty"`
	Action  string `yaml:"action,omitempty"`
	Level   string `yaml:"level,omitempty"`
	AgentID string `yaml:"agent_id,omitempty"`
}

// Matches returns true if the event satisfies this rule.
func (m MatchRule) Matches(e Event) bool {
	return matchesEvent(m.Source, m.Channel, m.Action, m.Level, m.AgentID, e)
}

// Webhook sends a POST with the event JSON to a URL.
type Webhook struct {
	URL     string            `yaml:"url"`
	Headers map[string]string `yaml:"headers,omitempty"`
}

// SlackConf configures Slack incoming webhook notifications.
type SlackConf struct {
	WebhookURL string `yaml:"webhook_url"`
}

// DiscordConf configures Discord webhook notifications.
type DiscordConf struct {
	WebhookURL string `yaml:"webhook_url"`
}

// DatabaseConf configures database storage for matched events.
type DatabaseConf struct {
	Driver string `yaml:"driver"` // sqlite (optional; defaults to sqlite)
	DSN    string `yaml:"dsn"`    // sqlite file path
	Table  string `yaml:"table"`  // table name (default "events")
}

// LoadConfig reads the config from a YAML file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &cfg, nil
}
