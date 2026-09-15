package state

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	_ "modernc.org/sqlite"
)

// SpecVersion is one committed snapshot of the spec file.
type SpecVersion struct {
	ID          int64
	Hash        string
	Content     []byte
	CommittedAt time.Time
	Message     string
}

// Apply is one agent run against a committed version. FinishedAt and
// ExitCode are nil until the run completes; nil forever means interrupted.
type Apply struct {
	ID         int64
	VersionID  int64
	Agent      string
	StartedAt  time.Time
	FinishedAt *time.Time
	ExitCode   *int
	LogPath    string
}

// DB wraps the SQLite handle and owns migrations.
type DB struct {
	db *sql.DB
}

// Open opens (creating if needed) the state database and applies pending migrations.
func Open(path string) (*DB, error) {
	d, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open state %s: %w", path, err)
	}
	d.SetMaxOpenConns(1) // sqlite: serialize internal access
	s := &DB{db: d}
	if err := s.migrate(); err != nil {
		d.Close()
		return nil, err
	}
	return s, nil
}

func (s *DB) Close() error { return s.db.Close() }

// migrations[n-1] upgrades schema version n-1 to n.
var migrations = []func(tx *sql.Tx) error{
	func(tx *sql.Tx) error { // v1
		for _, stmt := range []string{
			`CREATE TABLE spec_versions (
				id           INTEGER PRIMARY KEY AUTOINCREMENT,
				hash         TEXT NOT NULL,
				content      BLOB NOT NULL,
				committed_at TEXT NOT NULL,
				message      TEXT
			)`,
			`CREATE TABLE applies (
				id          INTEGER PRIMARY KEY AUTOINCREMENT,
				version_id  INTEGER NOT NULL REFERENCES spec_versions(id),
				agent       TEXT NOT NULL,
				started_at  TEXT NOT NULL,
				finished_at TEXT,
				exit_code   INTEGER,
				log_path    TEXT NOT NULL
			)`,
			`CREATE INDEX idx_applies_version ON applies(version_id)`,
		} {
			if _, err := tx.Exec(stmt); err != nil {
				return err
			}
		}
		return nil
	},
}

func (s *DB) migrate() error {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS meta (
		key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	var raw sql.NullString
	err := s.db.QueryRow(`SELECT value FROM meta WHERE key='schema_version'`).Scan(&raw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("migrate: %w", err)
	}
	current := 0
	if raw.Valid {
		if _, scanErr := fmt.Sscanf(raw.String, "%d", &current); scanErr != nil {
			return fmt.Errorf("migrate: corrupt schema_version %q", raw.String)
		}
	}
	if current > len(migrations) {
		return fmt.Errorf("migrate: state.db schema v%d is newer than this respex; upgrade respex", current)
	}
	for v := current + 1; v <= len(migrations); v++ {
		tx, txErr := s.db.Begin()
		if txErr != nil {
			return fmt.Errorf("migrate: %w", txErr)
		}
		if mErr := migrations[v-1](tx); mErr != nil {
			tx.Rollback()
			return fmt.Errorf("migrate to v%d: %w", v, mErr)
		}
		if _, uErr := tx.Exec(`INSERT INTO meta (key, value) VALUES ('schema_version', ?)
			ON CONFLICT(key) DO UPDATE SET value=excluded.value`, strconv.Itoa(v)); uErr != nil {
			tx.Rollback()
			return fmt.Errorf("migrate: %w", uErr)
		}
		if cErr := tx.Commit(); cErr != nil {
			return fmt.Errorf("migrate: %w", cErr)
		}
	}
	return nil
}

// SchemaVersion returns the current schema version.
func (s *DB) SchemaVersion() (int, error) {
	var raw string
	if err := s.db.QueryRow(`SELECT value FROM meta WHERE key='schema_version'`).Scan(&raw); err != nil {
		return 0, fmt.Errorf("schema version: %w", err)
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("schema version: corrupt value %q", raw)
	}
	return v, nil
}

type scanner interface{ Scan(dest ...any) error }

func parseRFC3339(s string) (time.Time, error) { return time.Parse(time.RFC3339, s) }

// InsertVersion stores a full spec snapshot and returns its id (displayed as vN).
func (s *DB) InsertVersion(hash string, content []byte, message string, now time.Time) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO spec_versions (hash, content, committed_at, message)
		VALUES (?, ?, ?, NULLIF(?, ''))`,
		hash, content, now.UTC().Format(time.RFC3339), message)
	if err != nil {
		return 0, fmt.Errorf("insert version: %w", err)
	}
	return res.LastInsertId()
}

func scanVersion(row scanner) (*SpecVersion, error) {
	var v SpecVersion
	var committed string
	var msg sql.NullString
	if err := row.Scan(&v.ID, &v.Hash, &v.Content, &committed, &msg); err != nil {
		return nil, err
	}
	at, err := parseRFC3339(committed)
	if err != nil {
		return nil, fmt.Errorf("spec_versions.committed_at: %w", err)
	}
	v.CommittedAt = at
	v.Message = msg.String
	return &v, nil
}

// LatestVersion returns the newest committed version, or nil when none exist.
func (s *DB) LatestVersion() (*SpecVersion, error) {
	row := s.db.QueryRow(`SELECT id, hash, content, committed_at, message
		FROM spec_versions ORDER BY id DESC LIMIT 1`)
	v, err := scanVersion(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("latest version: %w", err)
	}
	return v, nil
}

// GetVersion loads one stored snapshot by id.
func (s *DB) GetVersion(id int64) (*SpecVersion, error) {
	row := s.db.QueryRow(`SELECT id, hash, content, committed_at, message
		FROM spec_versions WHERE id = ?`, id)
	v, err := scanVersion(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("no such version v%d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get version %d: %w", id, err)
	}
	return v, nil
}

// ListVersions returns all versions, newest first.
func (s *DB) ListVersions() ([]SpecVersion, error) {
	rows, err := s.db.Query(`SELECT id, hash, content, committed_at, message
		FROM spec_versions ORDER BY id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	defer rows.Close()
	var out []SpecVersion
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *v)
	}
	return out, rows.Err()
}
