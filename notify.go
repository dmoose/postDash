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
	rules  []NotifyRule
	client *http.Client
}

// NewNotifier creates a Notifier from the given rules.
func NewNotifier(rules []NotifyRule) (*Notifier, error) {
	for i, r := range rules {
		if r.Webhook == nil && r.Slack == nil && r.Discord == nil {
			return nil, fmt.Errorf("rule %d (%s): no notification target configured", i, r.Name)
		}
	}
	return &Notifier{
		rules:  rules,
		client: &http.Client{Timeout: 5 * time.Second},
	}, nil
}

// Check evaluates an event against all rules and fires matching notifications.
func (n *Notifier) Check(evt Event) {
	for _, rule := range n.rules {
		if rule.Match.Matches(evt) {
			n.fire(rule, evt)
		}
	}
}

func (n *Notifier) fire(rule NotifyRule, evt Event) {
	if rule.Webhook != nil {
		n.fireWebhook(rule.Webhook, evt)
	}
	if rule.Slack != nil {
		n.fireSlack(rule.Slack, evt)
	}
	if rule.Discord != nil {
		n.fireDiscord(rule.Discord, evt)
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
