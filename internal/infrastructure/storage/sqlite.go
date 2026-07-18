package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps the SQLite database for persistence.
type DB struct {
	db *sql.DB
}

// New opens (or creates) the SQLite database at the given path.
func New(path string) (*DB, error) {
	if path == "" {
		path = "/data/reaplet.db"
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite %s: %w", path, err)
	}

	// WAL mode for concurrent reads
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("setting WAL mode: %w", err)
	}

	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrating schema: %w", err)
	}

	return &DB{db: db}, nil
}

func migrate(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS storage_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		node_name TEXT NOT NULL,
		timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		capacity_bytes INTEGER NOT NULL,
		allocated_bytes INTEGER NOT NULL,
		image_size_bytes INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_history_node_time ON storage_history(node_name, timestamp);

	CREATE TABLE IF NOT EXISTS alert_config (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		config_json TEXT NOT NULL DEFAULT '{}'
	);
	INSERT OR IGNORE INTO alert_config (id, config_json) VALUES (1, '{"warningPct":80,"criticalPct":90,"cooldownMin":15,"discord":{"enabled":false},"pushover":{"enabled":false}}');

	CREATE TABLE IF NOT EXISTS alert_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		node_name TEXT NOT NULL,
		level TEXT NOT NULL,
		usage_pct REAL NOT NULL,
		message TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_alerts_time ON alert_events(timestamp);

	CREATE TABLE IF NOT EXISTS image_first_seen (
		node_name TEXT NOT NULL,
		image_ref TEXT NOT NULL,
		first_seen DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (node_name, image_ref)
	);

	CREATE TABLE IF NOT EXISTS cleanup_config (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		config_json TEXT NOT NULL DEFAULT '{}'
	);
	INSERT OR IGNORE INTO cleanup_config (id, config_json) VALUES (1, '{"enabled":false,"intervalHours":6,"maxAgeDays":7,"maxSizeMB":500,"keepPatterns":[".*pause.*",".*coredns.*"],"maxPerCycle":5,"dryRun":true}');

	CREATE TABLE IF NOT EXISTS warm_list (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		image_ref TEXT NOT NULL UNIQUE,
		added_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := db.Exec(schema)
	return err
}

// Close closes the database.
func (d *DB) Close() error { return d.db.Close() }

// --- Storage History ---

// RecordHistory inserts a storage snapshot for a node.
func (d *DB) RecordHistory(nodeName string, capacity, allocated, imageSize int64) error {
	_, err := d.db.Exec(
		"INSERT INTO storage_history (node_name, capacity_bytes, allocated_bytes, image_size_bytes) VALUES (?, ?, ?, ?)",
		nodeName, capacity, allocated, imageSize,
	)
	return err
}

// HistoryPoint is a single history data point.
type HistoryPoint struct {
	Timestamp      time.Time `json:"timestamp"`
	CapacityBytes  int64     `json:"capacityBytes"`
	AllocatedBytes int64     `json:"allocatedBytes"`
	ImageSizeBytes int64     `json:"imageSizeBytes"`
}

// GetHistory returns history points for a node within a time range.
func (d *DB) GetHistory(nodeName string, since time.Time) ([]HistoryPoint, error) {
	rows, err := d.db.Query(
		"SELECT timestamp, capacity_bytes, allocated_bytes, image_size_bytes FROM storage_history WHERE node_name = ? AND timestamp > ? ORDER BY timestamp ASC",
		nodeName, since.UTC().Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []HistoryPoint
	for rows.Next() {
		var p HistoryPoint
		if err := rows.Scan(&p.Timestamp, &p.CapacityBytes, &p.AllocatedBytes, &p.ImageSizeBytes); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, rows.Err()
}

// PruneHistory removes data older than the given duration.
func (d *DB) PruneHistory(olderThan time.Duration) error {
	_, err := d.db.Exec("DELETE FROM storage_history WHERE timestamp < ?", time.Now().Add(-olderThan))
	return err
}

// --- Alert Config ---

// AlertConfig represents the alerting configuration.
type AlertConfig struct {
	WarningPct    int                      `json:"warningPct"`
	CriticalPct   int                      `json:"criticalPct"`
	CooldownMin   int                      `json:"cooldownMin"`
	NodeOverrides map[string]NodeThreshold `json:"nodeOverrides,omitempty"`
	Discord       DiscordConfig            `json:"discord"`
	Pushover      PushoverConfig           `json:"pushover"`
}

type NodeThreshold struct {
	WarningPct  int `json:"warningPct"`
	CriticalPct int `json:"criticalPct"`
}

type DiscordConfig struct {
	Enabled    bool   `json:"enabled"`
	WebhookURL string `json:"webhookUrl"`
}

type PushoverConfig struct {
	Enabled  bool   `json:"enabled"`
	AppToken string `json:"appToken"`
	UserKey  string `json:"userKey"`
}

func (d *DB) GetAlertConfig() (*AlertConfig, error) {
	var raw string
	err := d.db.QueryRow("SELECT config_json FROM alert_config WHERE id = 1").Scan(&raw)
	if err != nil {
		return nil, err
	}
	var cfg AlertConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (d *DB) SaveAlertConfig(cfg *AlertConfig) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	_, err = d.db.Exec("UPDATE alert_config SET config_json = ? WHERE id = 1", string(data))
	return err
}

// --- Alert Events ---

// AlertEvent is a historical alert firing.
type AlertEvent struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	NodeName  string    `json:"nodeName"`
	Level     string    `json:"level"`
	UsagePct  float64   `json:"usagePct"`
	Message   string    `json:"message"`
}

func (d *DB) RecordAlertEvent(nodeName, level string, usagePct float64, message string) error {
	_, err := d.db.Exec(
		"INSERT INTO alert_events (node_name, level, usage_pct, message) VALUES (?, ?, ?, ?)",
		nodeName, level, usagePct, message,
	)
	return err
}

func (d *DB) GetAlertEvents(limit int) ([]AlertEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := d.db.Query("SELECT id, timestamp, node_name, level, usage_pct, message FROM alert_events ORDER BY timestamp DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []AlertEvent
	for rows.Next() {
		var e AlertEvent
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.NodeName, &e.Level, &e.UsagePct, &e.Message); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// --- Image Age Tracking ---

func (d *DB) RecordImageSeen(nodeName, imageRef string) error {
	_, err := d.db.Exec(
		"INSERT OR IGNORE INTO image_first_seen (node_name, image_ref) VALUES (?, ?)",
		nodeName, imageRef,
	)
	return err
}

func (d *DB) GetImageAge(nodeName, imageRef string) (time.Time, error) {
	var t time.Time
	err := d.db.QueryRow(
		"SELECT first_seen FROM image_first_seen WHERE node_name = ? AND image_ref = ?",
		nodeName, imageRef,
	).Scan(&t)
	if err == sql.ErrNoRows {
		return time.Now(), nil
	}
	return t, err
}

func (d *DB) GetAllImageAges(nodeName string) (map[string]time.Time, error) {
	rows, err := d.db.Query("SELECT image_ref, first_seen FROM image_first_seen WHERE node_name = ?", nodeName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ages := make(map[string]time.Time)
	for rows.Next() {
		var ref string
		var t time.Time
		if err := rows.Scan(&ref, &t); err != nil {
			return nil, err
		}
		ages[ref] = t
	}
	return ages, rows.Err()
}

// --- Cleanup Config ---

type CleanupConfig struct {
	Enabled       bool     `json:"enabled"`
	IntervalHours int      `json:"intervalHours"`
	MaxAgeDays    int      `json:"maxAgeDays"`
	MaxSizeMB     int      `json:"maxSizeMB"`
	KeepPatterns  []string `json:"keepPatterns"`
	MaxPerCycle   int      `json:"maxPerCycle"`
	DryRun        bool     `json:"dryRun"`
}

func (d *DB) GetCleanupConfig() (*CleanupConfig, error) {
	var raw string
	err := d.db.QueryRow("SELECT config_json FROM cleanup_config WHERE id = 1").Scan(&raw)
	if err != nil {
		return nil, err
	}
	var cfg CleanupConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (d *DB) SaveCleanupConfig(cfg *CleanupConfig) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	_, err = d.db.Exec("UPDATE cleanup_config SET config_json = ? WHERE id = 1", string(data))
	return err
}

// --- Warm List ---

// WarmListEntry is a database row from warm_list.
type WarmListEntry struct {
	ID       int64     `json:"id"`
	ImageRef string    `json:"imageRef"`
	AddedAt  time.Time `json:"addedAt"`
}

// GetWarmList returns all entries in the warm list.
func (d *DB) GetWarmList() ([]WarmListEntry, error) {
	rows, err := d.db.Query("SELECT id, image_ref, added_at FROM warm_list ORDER BY added_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []WarmListEntry
	for rows.Next() {
		var e WarmListEntry
		if err := rows.Scan(&e.ID, &e.ImageRef, &e.AddedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// AddWarmListEntry inserts a new image into the warm list.
func (d *DB) AddWarmListEntry(imageRef string) (*WarmListEntry, error) {
	result, err := d.db.Exec("INSERT INTO warm_list (image_ref) VALUES (?)", imageRef)
	if err != nil {
		return nil, err
	}
	id, _ := result.LastInsertId()
	return &WarmListEntry{ID: id, ImageRef: imageRef, AddedAt: time.Now()}, nil
}

// DeleteWarmListEntry removes a warm list entry by ID.
func (d *DB) DeleteWarmListEntry(id int64) error {
	res, err := d.db.Exec("DELETE FROM warm_list WHERE id = ?", id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("warm list entry %d not found", id)
	}
	return nil
}

// --- Forecast History ---

// GetHistoryForForecast returns history points for the last N days for linear regression.
func (d *DB) GetHistoryForForecast(nodeName string, days int) ([]HistoryPoint, error) {
	since := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	return d.GetHistory(nodeName, since)
}
