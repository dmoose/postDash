package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the eventrelay configuration file.
type Config struct {
	Notify []NotifyRule `yaml:"notify"`
}

// NotifyRule defines when and where to send notifications.
type NotifyRule struct {
	Name    string     `yaml:"name"`              // human label for this rule
	Match   MatchRule  `yaml:"match"`             // event matching criteria
	Webhook *Webhook   `yaml:"webhook,omitempty"` // outbound webhook
	Slack   *SlackConf `yaml:"slack,omitempty"`   // Slack webhook
	Discord *DiscordConf `yaml:"discord,omitempty"` // Discord webhook
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
	if m.Source != "" && e.Source != m.Source {
		return false
	}
	if m.Channel != "" && e.Channel != m.Channel {
		return false
	}
	if m.Action != "" && e.Action != m.Action {
		return false
	}
	if m.Level != "" && e.Level != m.Level {
		return false
	}
	if m.AgentID != "" && e.AgentID != m.AgentID {
		return false
	}
	return true
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
