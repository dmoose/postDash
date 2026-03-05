package main

import (
	"encoding/json"
	"strings"
)

func matchesEvent(source, channel, action, level, agentID string, e Event) bool {
	if source != "" && e.Source != source {
		return false
	}
	if channel != "" && e.Channel != channel {
		return false
	}
	if action != "" && e.Action != action {
		return false
	}
	if level != "" && e.Level != level {
		return false
	}
	if agentID != "" && e.AgentID != agentID {
		return false
	}
	return true
}

func eventSourceMissing(raw []byte) bool {
	var check struct {
		Source string `json:"source"`
	}
	// Keep existing behavior: invalid JSON is handled later by publish/unmarshal.
	// This check only enforces required source when the body is decodable JSON.
	if err := json.Unmarshal(raw, &check); err != nil {
		return false
	}
	return strings.TrimSpace(check.Source) == ""
}
