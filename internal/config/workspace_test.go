package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateWorkspacePathInErrorMessage(t *testing.T) {
	dir := t.TempDir()
	// Create .bws directory to make it a workspace root
	if err := os.MkdirAll(filepath.Join(dir, ".bws"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".bws", "config.jsonc"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create 5 dummy files
	for i := 0; i < 5; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("file%d.txt", i)), []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// limit = 2, expect error with workspace path included
	err := ValidateWorkspace(dir, 2, false)
	if err == nil {
		t.Fatal("expected error due to file limit exceeded")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, dir) {
		t.Errorf("expected error message to contain workspace path %q, got: %s", dir, errMsg)
	}
	if !strings.Contains(errMsg, "contains more than 2 files") {
		t.Errorf("expected error message to mention file count limit, got: %s", errMsg)
	}

	// limit = -1, should suppress warning and succeed
	if err := ValidateWorkspace(dir, -1, false); err != nil {
		t.Errorf("expected limit -1 to suppress check, got error: %v", err)
	}

	// force = true, should bypass check and succeed
	if err := ValidateWorkspace(dir, 2, true); err != nil {
		t.Errorf("expected force=true to bypass check, got error: %v", err)
	}
}
