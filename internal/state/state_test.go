package state

import (
	"database/sql"
	"errors"
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

func TestFreshOpenCreatesSchemaV1(t *testing.T) {
	d := open(t)
	if v, err := d.SchemaVersion(); err != nil || v != 1 {
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

	if _, err := Open(p); err == nil || !strings.Contains(err.Error(), "migrate to v2") {
		t.Fatalf("Open with failing migration: err = %v, want migrate to v2 error", err)
	}

	migrations = orig
	d2, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d2.Close() })
	if v, err := d2.SchemaVersion(); err != nil || v != 1 {
		t.Fatalf("schema version after failed migration = %d, %v; want 1", v, err)
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

	id, err := d.InsertApply(vid, "fakeagent", now)
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
	id, _ := d.InsertApply(vid, "fakeagent", time.Now())
	d.FinishApply(id, 1, time.Now())
	if applied, _ := d.IsApplied(vid); applied {
		t.Fatal("failed apply must not count as applied")
	}
}
