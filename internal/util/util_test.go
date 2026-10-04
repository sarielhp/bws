package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUserTempDirIsolation(t *testing.T) {
	sub := "test_sub"
	dir, err := UserTempDir(sub)
	if err != nil {
		t.Fatalf("UserTempDir failed: %v", err)
	}
	defer os.RemoveAll(dir)

	fi, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat temp dir: %v", err)
	}
	if !fi.IsDir() {
		t.Fatalf("expected directory, got file")
	}

	base := UserTempDirBase()
	expectedPrefix := filepath.Join(os.TempDir(), fmt.Sprintf("bws-%d", os.Getuid()))
	if base != expectedPrefix {
		t.Errorf("UserTempDirBase() = %q, want %q", base, expectedPrefix)
	}

	// Verify either inside base or fallback
	if !strings.HasPrefix(dir, base) && !strings.HasPrefix(dir, os.TempDir()) {
		t.Errorf("temp dir %q not inside base %q or os.TempDir()", dir, base)
	}
}

func TestUserTempDirFallbackWhenBlocked(t *testing.T) {
	// Temporarily point TMPDIR to a clean temp dir
	tmpBase := t.TempDir()
	t.Setenv("TMPDIR", tmpBase)

	// Pre-create the per-user dir as a regular file with 0000 permissions
	blockedBase := filepath.Join(tmpBase, fmt.Sprintf("bws-%d", os.Getuid()))
	if err := os.WriteFile(blockedBase, []byte("blocked"), 0000); err != nil {
		t.Fatal(err)
	}

	// UserTempDir should gracefully fall back to os.TempDir()
	dir, err := UserTempDir("fallback")
	if err != nil {
		t.Fatalf("UserTempDir failed to fall back: %v", err)
	}
	defer os.RemoveAll(dir)

	if !strings.HasPrefix(dir, tmpBase) {
		t.Errorf("fallback dir %q not under TMPDIR %q", dir, tmpBase)
	}
	if strings.HasPrefix(dir, blockedBase+string(filepath.Separator)) {
		t.Errorf("fallback dir should not be inside blocked base: %s", dir)
	}
}
