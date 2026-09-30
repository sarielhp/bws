package sandbox

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProcessCopyPathsRejectsDotDotEscape(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "a", "h")
	sandboxDir := filepath.Join(home, ".sandbox", "sb")
	victim := filepath.Join(home, "Documents")
	source := filepath.Join(base, "Documents")
	for _, d := range []string{sandboxDir, victim, source} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	marker := filepath.Join(victim, "keep.txt")
	if err := os.WriteFile(marker, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)

	processCopyPaths([]string{home + "/../../Documents"}, sandboxDir)

	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("copy entry with .. escaped the staging dir and removed %s", victim)
	}
}
