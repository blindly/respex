package state

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func open(t *testing.T) *DB {
	t.Helper()
	d, err := Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestFreshOpenCreatesCurrentSchema(t *testing.T) {
	d := open(t)
	if v, err := d.SchemaVersion(); err != nil || v != 7 {
		t.Fatalf("schema version = %d, %v", v, err)
	}
}

func TestReopenIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	p := dir + "/state.db"
	d1, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	d1.Close()
	d2, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	d2.Close()
}

func TestOpenPercentPath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "dir%20name")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	d, err := Open(filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatalf("Open with %% in path: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	if _, err := d.InsertVersion("h1", []byte("one"), "msg", time.Now()); err != nil {
		t.Fatal(err)
	}
	v, err := d.LatestVersion()
	if err != nil || v == nil || v.Hash != "h1" || v.Message != "msg" {
		t.Fatalf("LatestVersion = %+v, %v", v, err)
	}
}

func TestVersionCRUD(t *testing.T) {
	d := open(t)
	now := time.Now()
	id1, err := d.InsertVersion("h1", []byte("one"), "first", now)
	if err != nil {
		t.Fatal(err)
	}
	id2, err := d.InsertVersion("h2", []byte("two"), "", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if id1 != 1 || id2 != 2 {
		t.Fatalf("ids = %d, %d", id1, id2)
	}
	last, err := d.LatestVersion()
	if err != nil || last == nil || last.ID != 2 || last.Hash != "h2" {
		t.Fatalf("LatestVersion = %+v, %v", last, err)
	}
	if last.Message != "" {
		t.Fatalf("empty message should read back empty, got %q", last.Message)
	}
	v1, err := d.GetVersion(1)
	if err != nil || string(v1.Content) != "one" || v1.Message != "first" {
		t.Fatalf("GetVersion = %+v, %v", v1, err)
	}
	if _, err := d.GetVersion(99); err == nil {
		t.Fatal("missing version should error")
	}
	all, err := d.ListVersions()
	if err != nil || len(all) != 2 || all[0].ID != 2 {
		t.Fatalf("ListVersions = %+v, %v", all, err)
	}
}

func TestLatestVersionNilWhenEmpty(t *testing.T) {
	d := open(t)
	v, err := d.LatestVersion()
	if err != nil || v != nil {
		t.Fatalf("LatestVersion = %+v, %v; want nil, nil", v, err)
	}
}

func TestMigrateLegacyDatabasePreservesHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
		`INSERT INTO meta VALUES ('schema_version', '1')`,
		`CREATE TABLE spec_versions (id INTEGER PRIMARY KEY AUTOINCREMENT, hash TEXT NOT NULL, content BLOB NOT NULL, committed_at TEXT NOT NULL, message TEXT)`,
		`CREATE TABLE applies (id INTEGER PRIMARY KEY AUTOINCREMENT, version_id INTEGER NOT NULL REFERENCES spec_versions(id), agent TEXT NOT NULL, started_at TEXT NOT NULL, finished_at TEXT, exit_code INTEGER, log_path TEXT NOT NULL)`,
		`CREATE INDEX idx_applies_version ON applies(version_id)`,
		`INSERT INTO spec_versions VALUES (1, 'hash', X'73706563', '2026-01-01T00:00:00Z', NULL)`,
		`INSERT INTO applies VALUES (1, 1, 'agent', '2026-01-01T00:00:00Z', NULL, NULL, '')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()
	migrated, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer migrated.Close()
	if version, err := migrated.SchemaVersion(); err != nil || version != 7 {
		t.Fatalf("schema version = %d, %v", version, err)
	}
	applies, err := migrated.ListApplies()
	if err != nil || len(applies) != 1 || applies[0].Outcome != "stale" {
		t.Fatalf("migrated applies = %+v, %v", applies, err)
	}
	if _, err := migrated.InsertRefine("agent", "unchanged", "hash", "hash", []byte("spec"), []byte("spec"), "", "", "", time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateCorruptSchemaVersion(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  string
	}{
		{"-1", "corrupt schema_version"},
		{"9", "newer"},
	} {
		t.Run(tc.value, func(t *testing.T) {
			dir := t.TempDir()
			p := dir + "/state.db"
			d, err := Open(p)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := d.db.Exec(`UPDATE meta SET value = ? WHERE key='schema_version'`, tc.value); err != nil {
				t.Fatal(err)
			}
			d.Close()
			if _, err := Open(p); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Open with schema_version %q: err = %v, want containing %q", tc.value, err, tc.want)
			}
		})
	}
}

func TestMigrateRollback(t *testing.T) {
	dir := t.TempDir()
	p := dir + "/state.db"
	d, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	d.Close()

	orig := migrations
	t.Cleanup(func() { migrations = orig })
	migrations = append(migrations, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`CREATE TABLE rollback_probe (x TEXT)`); err != nil {
			return err
		}
		return errors.New("boom")
	})

	if _, err := Open(p); err == nil || !strings.Contains(err.Error(), "migrate to v8") {
		t.Fatalf("Open with failing migration: err = %v, want migrate to v8 error", err)
	}

	migrations = orig
	d2, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d2.Close() })
	if v, err := d2.SchemaVersion(); err != nil || v != 7 {
		t.Fatalf("schema version after failed migration = %d, %v; want 7", v, err)
	}
	var n int
	if err := d2.db.QueryRow(`SELECT count(*) FROM sqlite_master
		WHERE type='table' AND name='rollback_probe'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal("failed migration's table should be rolled back")
	}
}

func TestApplyLifecycle(t *testing.T) {
	d := open(t)
	now := time.Now()
	vid, _ := d.InsertVersion("h1", []byte("one"), "", now)

	id, err := d.InsertApply(vid, "fakeagent", "", now)
	if err != nil || id != 1 {
		t.Fatalf("InsertApply = %d, %v", id, err)
	}
	if unfinished, err := d.HasUnfinishedApply(); err != nil || !unfinished {
		t.Fatalf("HasUnfinishedApply = %v, %v; want true", unfinished, err)
	}
	if applied, err := d.IsApplied(vid); err != nil || applied {
		t.Fatalf("IsApplied before finish = %v, %v; want false", applied, err)
	}
	if err := d.SetApplyLogPath(id, ".respex/logs/1-apply.log"); err != nil {
		t.Fatal(err)
	}
	if err := d.FinishApply(id, 0, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if applied, err := d.IsApplied(vid); err != nil || !applied {
		t.Fatalf("IsApplied after exit 0 = %v, %v; want true", applied, err)
	}
	if unfinished, _ := d.HasUnfinishedApply(); unfinished {
		t.Fatal("no unfinished applies should remain")
	}
	a, err := d.ListApplies()
	if err != nil || len(a) != 1 || a[0].ID != 1 || a[0].LogPath != ".respex/logs/1-apply.log" {
		t.Fatalf("ListApplies = %+v, %v", a, err)
	}
	if a[0].ExitCode == nil || *a[0].ExitCode != 0 || a[0].FinishedAt == nil {
		t.Fatalf("apply row incomplete: %+v", a[0])
	}
}

func TestFailedApplyDoesNotCount(t *testing.T) {
	d := open(t)
	vid, _ := d.InsertVersion("h1", []byte("one"), "", time.Now())
	id, _ := d.InsertApply(vid, "fakeagent", "", time.Now())
	d.FinishApply(id, 1, time.Now())
	if applied, _ := d.IsApplied(vid); applied {
		t.Fatal("failed apply must not count as applied")
	}
}

func TestListAppliesReadsInterruptedRow(t *testing.T) {
	d := open(t)
	vid, _ := d.InsertVersion("h1", []byte("one"), "", time.Now())
	if _, err := d.InsertApply(vid, "fakeagent", "", time.Now()); err != nil {
		t.Fatal(err)
	}
	a, err := d.ListApplies()
	if err != nil {
		t.Fatalf("ListApplies on interrupted row: %v", err)
	}
	if len(a) != 1 {
		t.Fatalf("len(ListApplies) = %d, want 1", len(a))
	}
	if a[0].FinishedAt != nil || a[0].ExitCode != nil || a[0].LogPath != "" {
		t.Fatalf("interrupted row = %+v; want nil FinishedAt, nil ExitCode, empty LogPath", a[0])
	}
}
