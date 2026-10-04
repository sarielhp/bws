package cli

import (
	"os"
	"path/filepath"
	"testing"

	"bws/internal/config"
)

func TestUndoRestoresAndForcesReTrust(t *testing.T) {
	tmpDir, cleanup := setupTestWorkspace(t)
	defer cleanup()

	cfgPath := filepath.Join(tmpDir, ".bws", "config.jsonc")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		t.Fatal(err)
	}

	// First write has no prior state; second write backs up the first.
	if err := config.WriteTrustedFile(cfgPath, []byte("{\"profiles\":[\"go\"]}\n")); err != nil {
		t.Fatal(err)
	}
	if err := config.WriteTrustedFile(cfgPath, []byte("{\"profiles\":[\"go\",\"git\"]}\n")); err != nil {
		t.Fatal(err)
	}

	// The local write is auto-trusted.
	if _, err := config.ReadTrustedFile(cfgPath); err != nil {
		t.Fatalf("written local config should be trusted: %v", err)
	}

	HandleUndo(false, true)

	got, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "{\"profiles\":[\"go\"]}\n" {
		t.Errorf("undo restored %q, want pre-write state", got)
	}

	// Restored content must no longer be trusted.
	if _, err := config.ReadTrustedFile(cfgPath); err == nil {
		t.Error("undo must leave the restored local config untrusted")
	}
}

func TestUndoIdempotent(t *testing.T) {
	tmpDir, cleanup := setupTestWorkspace(t)
	defer cleanup()

	cfgPath := filepath.Join(tmpDir, ".bws", "config.jsonc")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := config.WriteTrustedFile(cfgPath, []byte("{}\n")); err != nil {
		t.Fatal(err)
	}
	if err := config.WriteTrustedFile(cfgPath, []byte("{\"a\":1}\n")); err != nil {
		t.Fatal(err)
	}

	HandleUndo(false, true)
	HandleUndo(false, true)

	got, _ := os.ReadFile(cfgPath)
	if string(got) != "{}\n" {
		t.Errorf("second undo changed content to %q; should be idempotent", got)
	}
}

func TestUndoWithoutBackupFails(t *testing.T) {
	tmpDir, cleanup := setupTestWorkspace(t)
	defer cleanup()

	cfgPath := filepath.Join(tmpDir, ".bws", "config.jsonc")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := config.WriteTrustedFile(cfgPath, []byte("{}\n")); err != nil {
		t.Fatal(err)
	}
	// Only one write happened, so no backup exists.
	if config.HasBackup(cfgPath) {
		t.Fatal("unexpected backup after first write")
	}
}
