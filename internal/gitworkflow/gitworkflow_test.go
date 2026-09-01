package gitworkflow

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGitWorkflowHelpers(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize git repo
	if err := runCmd(tmpDir, "git", "init"); err != nil {
		t.Fatal(err)
	}
	if err := runCmd(tmpDir, "git", "config", "user.email", "test@example.com"); err != nil {
		// explicitly ignored in test
	}
	if err := runCmd(tmpDir, "git", "config", "user.name", "Test User"); err != nil {
		// explicitly ignored in test
	}

	root, err := getGitRoot()
	if err != nil || root == "" {
		t.Fatalf("getGitRoot failed on host: %v", err)
	}

	// Test dirty check
	testFile := filepath.Join(tmpDir, "hello.txt")
	os.WriteFile(testFile, []byte("world"), 0644)

	dirty, err := checkDirty(tmpDir)
	if err != nil {
		t.Fatalf("checkDirty failed: %v", err)
	}
	if !dirty {
		t.Errorf("expected dirty=true, got false")
	}

	// Commit file
	if err := runCmd(tmpDir, "git", "add", "hello.txt"); err != nil {
		// explicitly ignored in test
	}
	if err := runCmd(tmpDir, "git", "commit", "-m", "initial"); err != nil {
		// explicitly ignored in test
	}

	dirty, err = checkDirty(tmpDir)
	if err != nil || dirty {
		t.Errorf("expected clean repo, got dirty=%v, err=%v", dirty, err)
	}

	branch, err := getCurrentBranch(tmpDir)
	if err != nil || branch == "" {
		t.Errorf("getCurrentBranch failed: %v, got %q", err, branch)
	}
}
