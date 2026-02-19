package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Notifier evaluates events against rules and sends notifications.
type Notifier struct {
	rules    []NotifyRule
	client   *http.Client
	dbStores map[int]*DBStore // keyed by rule index
}

// NewNotifier creates a Notifier from the given rules.
func NewNotifier(rules []NotifyRule) (*Notifier, error) {
	n := &Notifier{
		rules:    rules,
		client:   &http.Client{Timeout: 5 * time.Second},
		dbStores: make(map[int]*DBStore),
	}

	for i, r := range rules {
		hasTarget := r.Webhook != nil || r.Slack != nil || r.Discord != nil || r.Database != nil
		if !hasTarget {
			return nil, fmt.Errorf("rule %d (%s): no notification target configured", i, r.Name)
		}
		if r.Database != nil {
			store, err := OpenDBStore(r.Database)
			if err != nil {
				return nil, fmt.Errorf("rule %d (%s): %w", i, r.Name, err)
			}
			n.dbStores[i] = store
			log.Printf("Database target ready: %s (%s → %s)", r.Name, r.Database.Driver, r.Database.DSN)
		}
	}

	return n, nil
}

// Close cleans up database connections.
func (n *Notifier) Close() {
	for _, store := range n.dbStores {
		store.Close()
	}
}

// Check evaluates an event against all rules and fires matching notifications.
func (n *Notifier) Check(evt Event) {
	for i, rule := range n.rules {
		if rule.Match.Matches(evt) {
			n.fire(i, rule, evt)
		}
	}
}

func (n *Notifier) fire(ruleIdx int, rule NotifyRule, evt Event) {
	if rule.Webhook != nil {
		n.fireWebhook(rule.Webhook, evt)
	}
	if rule.Slack != nil {
		n.fireSlack(rule.Slack, evt)
	}
	if rule.Discord != nil {
		n.fireDiscord(rule.Discord, evt)
	}
	if store, ok := n.dbStores[ruleIdx]; ok {
		store.Insert(evt)
	}
}

func (n *Notifier) fireWebhook(wh *Webhook, evt Event) {
	body, _ := json.Marshal(evt)
	req, err := http.NewRequest("POST", wh.URL, bytes.NewReader(body))
	if err != nil {
		log.Printf("webhook error: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range wh.Headers {
		req.Header.Set(k, v)
	}
	resp, err := n.client.Do(req)
	if err != nil {
		log.Printf("webhook %s error: %v", wh.URL, err)
		return
	}
	resp.Body.Close()
}

func (n *Notifier) fireSlack(cfg *SlackConf, evt Event) {
	text := formatNotification(evt)
	payload := map[string]string{"text": text}
	body, _ := json.Marshal(payload)

	resp, err := n.client.Post(cfg.WebhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("slack error: %v", err)
		return
	}
	resp.Body.Close()
}

func (n *Notifier) fireDiscord(cfg *DiscordConf, evt Event) {
	text := formatNotification(evt)
	payload := map[string]string{"content": text}
	body, _ := json.Marshal(payload)

	resp, err := n.client.Post(cfg.WebhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("discord error: %v", err)
		return
	}
	resp.Body.Close()
}

func formatNotification(evt Event) string {
	msg := fmt.Sprintf("[%s] %s", evt.Level, evt.Source)
	if evt.Channel != "" {
		msg += "/" + evt.Channel
	}
	if evt.Action != "" {
		msg += ": " + evt.Action
	}
	if evt.AgentID != "" {
		msg += fmt.Sprintf(" (agent: %s)", evt.AgentID)
	}
	if evt.DurationMS != nil {
		msg += fmt.Sprintf(" [%dms]", *evt.DurationMS)
	}
	if len(evt.Data) > 0 {
		data, _ := json.Marshal(evt.Data)
		msg += "\n```\n" + string(data) + "\n```"
	}
	return msg
}
