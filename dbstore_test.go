package main

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDBStoreInsertAndQuery(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	store, err := OpenDBStore(&DatabaseConf{
		Driver: "sqlite",
		DSN:    dbPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	dur := int64(42)
	store.Insert(Event{
		Seq:        1,
		Source:     "test",
		Channel:    "ops",
		Action:     "deploy",
		Level:      "info",
		AgentID:    "agent-1",
		DurationMS: &dur,
		Data:       map[string]any{"env": "prod"},
		TS:         time.Now(),
	})

	store.Insert(Event{
		Seq:    2,
		Source: "test",
		Level:  "error",
		Action: "crash",
		TS:     time.Now(),
	})

	// Verify by querying directly
	db, _ := sql.Open("sqlite", dbPath)
	defer db.Close()

	var count int
	db.QueryRow("SELECT COUNT(*) FROM events").Scan(&count)
	if count != 2 {
		t.Fatalf("expected 2 rows, got %d", count)
	}

	var source, level, action, dataStr string
	var durationMS *int64
	db.QueryRow("SELECT source, level, action, duration_ms, data FROM events WHERE seq = 1").
		Scan(&source, &level, &action, &durationMS, &dataStr)

	if source != "test" || level != "info" || action != "deploy" {
		t.Errorf("unexpected values: source=%s level=%s action=%s", source, level, action)
	}
	if durationMS == nil || *durationMS != 42 {
		t.Errorf("expected duration_ms 42, got %v", durationMS)
	}

	var data map[string]any
	json.Unmarshal([]byte(dataStr), &data)
	if data["env"] != "prod" {
		t.Errorf("expected data.env=prod, got %v", data["env"])
	}

	// Test error row has no duration
	db.QueryRow("SELECT duration_ms FROM events WHERE seq = 2").Scan(&durationMS)
	if durationMS != nil {
		t.Errorf("expected nil duration_ms for error event, got %v", *durationMS)
	}
}

func TestDBStoreCustomTable(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	store, err := OpenDBStore(&DatabaseConf{
		Driver: "sqlite",
		DSN:    dbPath,
		Table:  "audit_log",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	store.Insert(Event{Seq: 1, Source: "test", Level: "info", TS: time.Now()})

	db, _ := sql.Open("sqlite", dbPath)
	defer db.Close()

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM audit_log").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected 1 row in audit_log, got %d", count)
	}

	// events table should not exist
	err = db.QueryRow("SELECT COUNT(*) FROM events").Scan(&count)
	if err == nil {
		t.Error("expected events table to not exist")
	}
}

func TestDBStoreFileCreated(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "new.db")

	store, err := OpenDBStore(&DatabaseConf{DSN: dbPath})
	if err != nil {
		t.Fatal(err)
	}
	store.Close()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("expected database file to be created")
	}
}
