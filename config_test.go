package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigWithServerSection(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "test.yaml")

	yaml := `
server:
  port: 8080
  bind: 0.0.0.0
  token: mysecret
  buffer: 5000
  log: /tmp/events.jsonl

notify:
  - name: test rule
    match:
      level: error
    webhook:
      url: http://example.com/hook
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Server == nil {
		t.Fatal("expected server config to be parsed")
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Server.Bind != "0.0.0.0" {
		t.Errorf("expected bind 0.0.0.0, got %s", cfg.Server.Bind)
	}
	if cfg.Server.Token != "mysecret" {
		t.Errorf("expected token mysecret, got %s", cfg.Server.Token)
	}
	if cfg.Server.Buffer != 5000 {
		t.Errorf("expected buffer 5000, got %d", cfg.Server.Buffer)
	}
	if cfg.Server.Log != "/tmp/events.jsonl" {
		t.Errorf("expected log path, got %s", cfg.Server.Log)
	}
	if len(cfg.Notify) != 1 {
		t.Errorf("expected 1 notify rule, got %d", len(cfg.Notify))
	}
}

func TestLoadConfigWithoutServerSection(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "test.yaml")

	yaml := `
notify:
  - name: errors
    match:
      level: error
    webhook:
      url: http://example.com/hook
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Server != nil {
		t.Error("expected nil server config when section omitted")
	}
	if len(cfg.Notify) != 1 {
		t.Errorf("expected 1 notify rule, got %d", len(cfg.Notify))
	}
}
