package gitworkflow

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestExportFailureDoesNotFetchOrRemoveClone(t *testing.T) {
	clone := t.TempDir()
	marker := filepath.Join(clone, "change")
	if err := os.WriteFile(marker, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	expected := errors.New("export failed")
	err := exportAndFetch(t.TempDir(), clone, "branch", func(string, string, io.Writer) error { return expected })
	if !errors.Is(err, expected) {
		t.Fatalf("failure swallowed: %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("clone removed", err)
	}
}

func TestConfigCopyRejectsEscapingDestination(t *testing.T) {
	src, dest, outside := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(src, ".env"), []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "marker"), filepath.Join(dest, ".env")); err != nil {
		t.Fatal(err)
	}
	if err := copyConfigFiles(src, dest); err == nil {
		t.Fatal("escaping destination accepted")
	}
	if _, err := os.Stat(filepath.Join(outside, "marker")); !os.IsNotExist(err) {
		t.Fatal("outside file changed")
	}
}

func TestCopyConfigFilesPreservesSecurePerms(t *testing.T) {
	src, dest := t.TempDir(), t.TempDir()
	envPath := filepath.Join(src, ".env")
	envLocalPath := filepath.Join(src, ".env.local")
	if err := os.WriteFile(envPath, []byte("SECRET=123"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envLocalPath, []byte("TOKEN=abc"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := copyConfigFiles(src, dest); err != nil {
		t.Fatalf("copyConfigFiles failed: %v", err)
	}

	destEnv, err := os.Stat(filepath.Join(dest, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if destEnv.Mode().Perm() != 0600 {
		t.Errorf("staged .env perm = %o, want 0600", destEnv.Mode().Perm())
	}

	destEnvLocal, err := os.Stat(filepath.Join(dest, ".env.local"))
	if err != nil {
		t.Fatal(err)
	}
	if destEnvLocal.Mode().Perm() != 0600 {
		t.Errorf("staged .env.local perm = %o, want 0600", destEnvLocal.Mode().Perm())
	}
}
