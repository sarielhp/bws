package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsSameDirectory(t *testing.T) {
	dir := t.TempDir()
	child := filepath.Join(dir, "child")
	if err := os.Mkdir(child, 0755); err != nil {
		t.Fatal(err)
	}

	if !isSameDirectory(dir, dir) {
		t.Errorf("expected %s and %s to be same directory", dir, dir)
	}
	if !isSameDirectory(dir, dir+string(filepath.Separator)) {
		t.Errorf("expected trailing slash to be normalized as same directory")
	}
	if isSameDirectory(dir, child) {
		t.Errorf("expected parent and child not to be same directory")
	}
	if isSameDirectory("", dir) {
		t.Errorf("expected empty string not to match")
	}
}

func TestFindWorkspaceForPath(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".bws", "config.jsonc")
	legacyPath := filepath.Join(dir, ".bws.jsonc")

	if got := findWorkspaceForPath(configPath); got != dir {
		t.Errorf("expected %s, got %s", dir, got)
	}
	if got := findWorkspaceForPath(legacyPath); got != dir {
		t.Errorf("expected %s, got %s", dir, got)
	}
}

func TestFormatWorkspace(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	formattedCwd := FormatWorkspace(cwd)
	if strings.Contains(formattedCwd, "(not current directory)") {
		t.Errorf("expected current directory not to contain notice, got %s", formattedCwd)
	}

	otherDir := t.TempDir()
	formattedOther := FormatWorkspace(otherDir)
	if !strings.Contains(formattedOther, "(not current directory)") {
		t.Errorf("expected other directory to contain (not current directory), got %s", formattedOther)
	}

	header := FormatWorkspaceHeader(otherDir)
	if !strings.HasPrefix(header, "Workspace: ") {
		t.Errorf("expected header to start with 'Workspace: ', got %s", header)
	}
}
