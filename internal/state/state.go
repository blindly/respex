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
	d, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("open state %s: %w", path, err)
	}
	d.SetMaxOpenConns(1) // sqlite: serialize internal access
	if err := d.Ping(); err != nil {
		d.Close()
		return nil, fmt.Errorf("open state %s: %w", path, err)
	}
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
		var scanErr error
		if current, scanErr = strconv.Atoi(raw.String); scanErr != nil || current < 0 {
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
		return nil, fmt.Errorf("latest version: scan version: %w", err)
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
		return nil, fmt.Errorf("get version %d: scan version: %w", id, err)
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
			return nil, fmt.Errorf("list versions: scan version: %w", err)
		}
		out = append(out, *v)
	}
	if rErr := rows.Err(); rErr != nil {
		return nil, fmt.Errorf("list versions: %w", rErr)
	}
	return out, nil
}

// InsertApply records the start of an apply run; log_path starts empty and is
// filled by SetApplyLogPath once the row id has named the file.
func (s *DB) InsertApply(versionID int64, agent string, now time.Time) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO applies
		(version_id, agent, started_at, finished_at, exit_code, log_path)
		VALUES (?, ?, ?, NULL, NULL, '')`, versionID, agent, now.UTC().Format(time.RFC3339))
	if err != nil {
		return 0, fmt.Errorf("insert apply: %w", err)
	}
	return res.LastInsertId()
}

// SetApplyLogPath records the (repo-relative) log path for an apply row.
func (s *DB) SetApplyLogPath(id int64, logPath string) error {
	_, err := s.db.Exec(`UPDATE applies SET log_path = ? WHERE id = ?`, logPath, id)
	if err != nil {
		return fmt.Errorf("set apply log path: %w", err)
	}
	return nil
}

// FinishApply stamps the outcome of an apply run.
func (s *DB) FinishApply(id int64, exitCode int, now time.Time) error {
	_, err := s.db.Exec(`UPDATE applies SET finished_at = ?, exit_code = ? WHERE id = ?`,
		now.UTC().Format(time.RFC3339), exitCode, id)
	if err != nil {
		return fmt.Errorf("finish apply: %w", err)
	}
	return nil
}

func scanApply(row scanner) (*Apply, error) {
	var a Apply
	var started string
	var finished sql.NullString
	var exit sql.NullInt64
	if err := row.Scan(&a.ID, &a.VersionID, &a.Agent, &started, &finished, &exit, &a.LogPath); err != nil {
		return nil, err
	}
	at, err := parseRFC3339(started)
	if err != nil {
		return nil, fmt.Errorf("applies.started_at: %w", err)
	}
	a.StartedAt = at
	if finished.Valid {
		ft, err := parseRFC3339(finished.String)
		if err != nil {
			return nil, fmt.Errorf("applies.finished_at: %w", err)
		}
		a.FinishedAt = &ft
	}
	if exit.Valid {
		c := int(exit.Int64)
		a.ExitCode = &c
	}
	return &a, nil
}

// IsApplied reports whether versionID has an apply row with exit_code = 0.
func (s *DB) IsApplied(versionID int64) (bool, error) {
	var exists bool
	if err := s.db.QueryRow(`SELECT EXISTS(
		SELECT 1 FROM applies WHERE version_id = ? AND exit_code = 0)`, versionID).Scan(&exists); err != nil {
		return false, fmt.Errorf("is applied: %w", err)
	}
	return exists, nil
}

// HasUnfinishedApply reports whether any apply row lacks an outcome.
func (s *DB) HasUnfinishedApply() (bool, error) {
	var exists bool
	if err := s.db.QueryRow(`SELECT EXISTS(
		SELECT 1 FROM applies WHERE finished_at IS NULL)`).Scan(&exists); err != nil {
		return false, fmt.Errorf("unfinished apply: %w", err)
	}
	return exists, nil
}

// ListApplies returns all apply rows, newest first.
func (s *DB) ListApplies() ([]Apply, error) {
	rows, err := s.db.Query(`SELECT id, version_id, agent, started_at, finished_at, exit_code, log_path
		FROM applies ORDER BY id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list applies: %w", err)
	}
	defer rows.Close()
	var out []Apply
	for rows.Next() {
		a, err := scanApply(rows)
		if err != nil {
			return nil, fmt.Errorf("list applies: scan apply: %w", err)
		}
		out = append(out, *a)
	}
	if rErr := rows.Err(); rErr != nil {
		return nil, fmt.Errorf("list applies: %w", rErr)
	}
	return out, nil
}
