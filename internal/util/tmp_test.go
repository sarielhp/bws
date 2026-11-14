package util

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUserTempDirUsesProjectTree(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	dir, err := UserTempDir("stage")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(os.Getenv("TMPDIR"), userName(), "bws", "stage") + string(filepath.Separator) + "stage_"
	if !strings.HasPrefix(dir, want) {
		t.Errorf("UserTempDir = %q, want prefix %q", dir, want)
	}
	for _, d := range []string{filepath.Dir(dir), filepath.Dir(filepath.Dir(dir)), dir} {
		fi, statErr := os.Stat(d)
		if statErr != nil || fi.Mode().Perm() != 0o700 {
			t.Errorf("%s: mode %v, err %v; want 0700", d, fi.Mode().Perm(), statErr)
		}
	}
	f, err := UserTempFile("learn", "trace-*.log")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if filepath.Dir(f.Name()) != TempPath("learn") {
		t.Errorf("UserTempFile = %q, want it in %q", f.Name(), TempPath("learn"))
	}
}

func TestTempTreeRefusesUnsafeDirectories(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	userDir := filepath.Join(tmp, userName())

	// A plain file where the user directory belongs is refused, not replaced.
	if err := os.WriteFile(userDir, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := UserTempDir("stage"); err == nil {
		t.Error("a file in place of the user directory was accepted")
	}
	os.Remove(userDir)

	// A user directory of ours that is group-writable (mkdir -p under umask
	// 002) is used as it is; bws does not change its mode.
	if err := os.Mkdir(userDir, 0o775); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(userDir, 0o775); err != nil {
		t.Fatal(err)
	}
	if _, err := UserTempDir("stage"); err != nil {
		t.Fatalf("a group-writable user directory of ours was refused: %v", err)
	}
	if fi, _ := os.Stat(userDir); fi.Mode().Perm() != 0o775 {
		t.Errorf("user directory mode changed to %v", fi.Mode().Perm())
	}

	// One writable by other users is refused unless it is sticky.
	if err := os.Chmod(userDir, 0o777); err != nil {
		t.Fatal(err)
	}
	if _, err := UserTempDir("stage"); err == nil {
		t.Error("a world-writable, non-sticky user directory was accepted")
	}
	if err := os.Chmod(userDir, 0o777|os.ModeSticky); err != nil {
		t.Fatal(err)
	}
	if _, err := UserTempDir("stage"); err != nil {
		t.Errorf("a sticky user directory was refused: %v", err)
	}
	if err := os.Chmod(userDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// A project directory of ours with loose permissions is tightened.
	if err := os.RemoveAll(filepath.Join(userDir, "bws")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(userDir, "bws"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := UserTempDir("stage"); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(filepath.Join(userDir, "bws")); fi.Mode().Perm() != 0o700 {
		t.Errorf("project directory mode %v, want 0700", fi.Mode().Perm())
	}
}
