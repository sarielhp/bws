package learn

import (
	"path/filepath"
	"testing"
)

func TestShouldFilterAccess_AncestorFiltering(t *testing.T) {
	homeDir := "/home/testuser"
	courseDir := filepath.Join(homeDir, "course")
	notesDir := filepath.Join(courseDir, "notes")
	workDir := filepath.Join(notesDir, "06_verify")
	prefixFile := filepath.Join(notesDir, "prefix.tex")

	// 1. Ancestor directories should be filtered out
	if !ShouldFilterAccess(notesDir, AccessRead, workDir, homeDir) {
		t.Errorf("expected parent directory %s to be filtered, but it was not", notesDir)
	}
	if !ShouldFilterAccess(courseDir, AccessRead, workDir, homeDir) {
		t.Errorf("expected grandparent directory %s to be filtered, but it was not", courseDir)
	}

	// 2. Regular file in ancestor directory (pinhole candidate) must NOT be filtered
	if ShouldFilterAccess(prefixFile, AccessRead, workDir, homeDir) {
		t.Errorf("expected ancestor file %s to NOT be filtered, but it was filtered", prefixFile)
	}

	// 3. Workspace-internal file should be filtered (already mounted by CWD)
	workFile := filepath.Join(workDir, "verify.tex")
	if !ShouldFilterAccess(workFile, AccessRead, workDir, homeDir) {
		t.Errorf("expected workspace internal file %s to be filtered, but it was not", workFile)
	}
}
