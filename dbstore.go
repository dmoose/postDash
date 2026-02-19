package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	_ "modernc.org/sqlite"
)

// DBStore handles inserting events into a database.
type DBStore struct {
	db    *sql.DB
	table string
	mu    sync.Mutex
}

// OpenDBStore connects to a database and ensures the events table exists.
func OpenDBStore(cfg *DatabaseConf) (*DBStore, error) {
	driver := cfg.Driver
	if driver == "" {
		driver = "sqlite"
	}
	table := cfg.Table
	if table == "" {
		table = "events"
	}

	db, err := sql.Open(driver, cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	// Create table if it doesn't exist.
	// Use TEXT for timestamps and JSON data for maximum portability across DBs.
	createSQL := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		seq        INTEGER,
		source     TEXT NOT NULL DEFAULT '',
		channel    TEXT NOT NULL DEFAULT '',
		action     TEXT NOT NULL DEFAULT '',
		level      TEXT NOT NULL DEFAULT 'info',
		agent_id   TEXT NOT NULL DEFAULT '',
		duration_ms INTEGER,
		data       TEXT,
		ts         TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`, table)

	if _, err := db.Exec(createSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating table: %w", err)
	}

	// Create indexes for common queries
	for _, col := range []string{"source", "level", "channel", "ts"} {
		idx := fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_%s ON %s(%s)", table, col, table, col)
		db.Exec(idx) // best-effort, some DBs may not support IF NOT EXISTS on indexes
	}

	return &DBStore{db: db, table: table}, nil
}

// Insert stores an event in the database.
func (s *DBStore) Insert(evt Event) {
	var dataJSON *string
	if len(evt.Data) > 0 {
		b, _ := json.Marshal(evt.Data)
		str := string(b)
		dataJSON = &str
	}

	var durationMS *int64
	if evt.DurationMS != nil {
		durationMS = evt.DurationMS
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	insertSQL := fmt.Sprintf(
		`INSERT INTO %s (seq, source, channel, action, level, agent_id, duration_ms, data, ts)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.table,
	)

	_, err := s.db.Exec(insertSQL,
		evt.Seq, evt.Source, evt.Channel, evt.Action, evt.Level,
		evt.AgentID, durationMS, dataJSON, evt.TS.Format("2006-01-02T15:04:05.000Z07:00"),
	)
	if err != nil {
		log.Printf("db insert error: %v", err)
	}
}

// Close closes the database connection.
func (s *DBStore) Close() error {
	return s.db.Close()
}
