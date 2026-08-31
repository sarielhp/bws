package learn

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCanonicalPath(t *testing.T) {
	homeDir := "/home/testuser"

	tests := []struct {
		input string
		want  string
	}{
		{"~/.cargo/bin", "/home/testuser/.cargo/bin"},
		{"@@HOME@@/.cargo/bin", "/home/testuser/.cargo/bin"},
		{"/usr/bin", "/usr/bin"},
		{"/usr/local/bin/", "/usr/local/bin"},
		{"~", "/home/testuser"},
		{"@@HOME@@", "/home/testuser"},
	}

	for _, tt := range tests {
		got := CanonicalPath(tt.input, homeDir)
		if got != tt.want {
			t.Errorf("CanonicalPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIsPathCovered(t *testing.T) {
	homeDir := "/home/testuser"
	configPaths := []string{
		"~/.local/bin",
		"/opt/mytools/bin",
	}

	tests := []struct {
		dir  string
		want bool
	}{
		{"/usr/bin", true},
		{"/bin", true},
		{"/usr/local/bin", true},
		{"~/.local/bin", true},
		{"/home/testuser/.local/bin", true},
		{"@@HOME@@/.local/bin", true},
		{"~/bin", true},
		{"/opt/mytools/bin", true},
		{"~/.cargo/bin", false},
		{"/opt/custom/bin", false},
	}

	for _, tt := range tests {
		got := IsPathCovered(tt.dir, configPaths, homeDir)
		if got != tt.want {
			t.Errorf("IsPathCovered(%q) = %v, want %v", tt.dir, got, tt.want)
		}
	}
}

func TestResolveBinaryDir(t *testing.T) {
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "custom", "bin")
	os.MkdirAll(binDir, 0755)
	binPath := filepath.Join(binDir, "myexec")
	os.WriteFile(binPath, []byte("#!/bin/sh\necho ok\n"), 0755)

	// Relative path resolution
	resolved, err := ResolveBinaryDir(binPath, "", tmpDir)
	if err != nil {
		t.Fatalf("ResolveBinaryDir failed: %v", err)
	}
	if resolved != "~/custom/bin" {
		t.Errorf("resolved = %q, want '~/custom/bin'", resolved)
	}
}
