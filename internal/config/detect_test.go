package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectFeatures(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(dir string)
		dirName  string
		expected ProjectFeatures
	}{
		{
			name: "Go Project",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)
				os.MkdirAll(filepath.Join(dir, "pkg"), 0755)
				os.WriteFile(filepath.Join(dir, "pkg", "main.go"), []byte("package main"), 0644)
			},
			expected: ProjectFeatures{
				HasGo:     true,
				EnableSSH: true,
			},
		},
		{
			name: "Python Project with uv",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]"), 0644)
				os.WriteFile(filepath.Join(dir, "uv.lock"), []byte(""), 0644)
			},
			expected: ProjectFeatures{
				HasPython: true,
				EnableSSH: true,
			},
		},
		{
			name: "Rust and Node Project",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]"), 0644)
				os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644)
			},
			expected: ProjectFeatures{
				HasRust:   true,
				HasNode:   true,
				EnableSSH: true,
			},
		},
		{
			name: "OpenCode directory",
			setup: func(dir string) {
				os.MkdirAll(filepath.Join(dir, ".open-mem"), 0755)
			},
			expected: ProjectFeatures{
				HasOpenCode: true,
				EnableSSH:   true,
			},
		},
		{
			name: "LaTeX Document Project",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "paper.tex"), []byte("\\documentclass{article}"), 0644)
				os.WriteFile(filepath.Join(dir, "references.bib"), []byte("@article{}"), 0644)
			},
			expected: ProjectFeatures{
				HasLatex:  true,
				EnableSSH: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "bw_detect_test_*")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			targetDir := tmpDir
			if tt.dirName != "" {
				targetDir = filepath.Join(tmpDir, tt.dirName)
				os.MkdirAll(targetDir, 0755)
			}

			if tt.setup != nil {
				tt.setup(targetDir)
			}

			got, err := DetectFeatures(targetDir)
			if err != nil {
				t.Fatalf("DetectFeatures failed: %v", err)
			}

			if got.HasGo != tt.expected.HasGo {
				t.Errorf("HasGo = %v, want %v", got.HasGo, tt.expected.HasGo)
			}
			if got.HasPython != tt.expected.HasPython {
				t.Errorf("HasPython = %v, want %v", got.HasPython, tt.expected.HasPython)
			}
			if got.HasRust != tt.expected.HasRust {
				t.Errorf("HasRust = %v, want %v", got.HasRust, tt.expected.HasRust)
			}
			if got.HasNode != tt.expected.HasNode {
				t.Errorf("HasNode = %v, want %v", got.HasNode, tt.expected.HasNode)
			}
			if got.HasOpenCode != tt.expected.HasOpenCode {
				t.Errorf("HasOpenCode = %v, want %v", got.HasOpenCode, tt.expected.HasOpenCode)
			}
			if got.EnableSSH != tt.expected.EnableSSH {
				t.Errorf("EnableSSH = %v, want %v", got.EnableSSH, tt.expected.EnableSSH)
			}
		})
	}
}
