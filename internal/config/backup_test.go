package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestIsConfigFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cases := []struct {
		path string
		want bool
	}{
		{filepath.Join(home, "proj", ".bws", "config.jsonc"), true},
		{filepath.Join(home, "proj", ".bws", "config.json"), true},
		{filepath.Join(home, "proj", ".bws.jsonc"), true},
		{filepath.Join(home, ".config", "bws", "config.jsonc"), true},
		{filepath.Join(home, "proj", ".bws", "profiles", "go.json"), false},
		{filepath.Join(home, "proj", "config.jsonc"), false},
		{filepath.Join(home, "proj", ".bws", "config.jsonc.bak"), false},
	}
	for _, c := range cases {
		if got := IsConfigFile(c.path); got != c.want {
			t.Errorf("IsConfigFile(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestBackupConfigLocal(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, ".bws", "config.jsonc")
	writeFile(t, cfg, `{"profiles":["go"]}`)

	if err := BackupConfig(cfg); err != nil {
		t.Fatalf("BackupConfig: %v", err)
	}
	got, err := os.ReadFile(cfg + BackupSuffix)
	if err != nil {
		t.Fatalf("reading backup: %v", err)
	}
	if string(got) != `{"profiles":["go"]}` {
		t.Errorf("backup content = %q", got)
	}

	// Second backup overwrites with the newer state.
	writeFile(t, cfg, `{"profiles":["go","git"]}`)
	if err := BackupConfig(cfg); err != nil {
		t.Fatal(err)
	}
	got, _ = os.ReadFile(cfg + BackupSuffix)
	if string(got) != `{"profiles":["go","git"]}` {
		t.Errorf("backup not overwritten, got %q", got)
	}
}

func TestBackupConfigIgnoresNonConfigAndAbsent(t *testing.T) {
	dir := t.TempDir()
	profile := filepath.Join(dir, ".bws", "profiles", "go.json")
	writeFile(t, profile, `{"name":"go"}`)
	if err := BackupConfig(profile); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(profile + BackupSuffix); !os.IsNotExist(err) {
		t.Errorf("profile file must not be backed up")
	}

	// Absent config: no error, and any stale backup is cleared.
	cfg := filepath.Join(dir, ".bws", "config.jsonc")
	writeFile(t, cfg+BackupSuffix, `stale`)
	if err := BackupConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cfg + BackupSuffix); !os.IsNotExist(err) {
		t.Errorf("stale backup should be removed when config is absent")
	}
}

func TestRestoreBackup(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, ".bws", "config.jsonc")
	writeFile(t, cfg, `{"v":1}`)
	if err := BackupConfig(cfg); err != nil {
		t.Fatal(err)
	}
	writeFile(t, cfg, `{"v":2}`)

	if !HasBackup(cfg) {
		t.Fatal("HasBackup = false, want true")
	}
	if err := RestoreBackup(cfg); err != nil {
		t.Fatalf("RestoreBackup: %v", err)
	}
	got, _ := os.ReadFile(cfg)
	if string(got) != `{"v":1}` {
		t.Errorf("restored content = %q, want v1", got)
	}
	// Backup retained: undo is idempotent.
	if !HasBackup(cfg) {
		t.Error("backup should be retained after restore")
	}
	if err := RestoreBackup(cfg); err != nil {
		t.Errorf("second restore should succeed: %v", err)
	}
	got, _ = os.ReadFile(cfg)
	if string(got) != `{"v":1}` {
		t.Errorf("second restore content = %q", got)
	}
}

func TestRestoreBackupMissing(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, ".bws", "config.jsonc")
	writeFile(t, cfg, `{}`)
	if HasBackup(cfg) {
		t.Fatal("HasBackup = true, want false")
	}
	if err := RestoreBackup(cfg); err == nil {
		t.Error("RestoreBackup without backup should error")
	}
}

func TestBackupInvisibleToWorkspaceDiscovery(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, ".bws", "config.jsonc")
	writeFile(t, cfg, `{"profiles":["go"]}`)
	if err := BackupConfig(cfg); err != nil {
		t.Fatal(err)
	}
	// Remove the real config; only the backup remains.
	if err := os.Remove(cfg); err != nil {
		t.Fatal(err)
	}

	root, found := FindWorkspaceRoot(dir)
	if found == cfg+BackupSuffix {
		t.Fatalf("backup file leaked into discovery: %s", found)
	}
	// With no real config, discovery falls back to the start directory.
	if found != filepath.Join(dir, ".bws", "config.jsonc") {
		t.Errorf("discovery returned %q (root %q); backup should be invisible", found, root)
	}
}

func TestDisplayPathShortensHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	got := DisplayPath(filepath.Join(home, ".bws", "config.jsonc.bak"))
	if got != "~/.bws/config.jsonc.bak" {
		t.Errorf("DisplayPath = %q, want ~/.bws/config.jsonc.bak", got)
	}
	if outside := DisplayPath("/etc/bws.conf"); outside != "/etc/bws.conf" {
		t.Errorf("DisplayPath outside home = %q, want unchanged", outside)
	}
}
