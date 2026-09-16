package config

import (
	"bws/internal/detect"
	"path/filepath"
	"strings"
)

// ProjectFeatures represents the detected language stacks and capabilities for a workspace.
type ProjectFeatures struct {
	HasGo       bool
	HasPython   bool
	HasRust     bool
	HasNode     bool
	HasLatex    bool
	HasOpenCode bool
	EnableSSH   bool
}

// AnyDetected returns true if any language stack or toolchain feature is detected.
func (pf ProjectFeatures) AnyDetected() bool {
	return pf.HasGo || pf.HasPython || pf.HasRust || pf.HasNode || pf.HasLatex || pf.HasOpenCode
}

// DetectedStacks returns human-readable names of all detected stacks.
func (pf ProjectFeatures) DetectedStacks() []string {
	var detected []string
	if pf.HasGo {
		detected = append(detected, "Go")
	}
	if pf.HasPython {
		detected = append(detected, "Python/UV")
	}
	if pf.HasRust {
		detected = append(detected, "Rust")
	}
	if pf.HasNode {
		detected = append(detected, "Node")
	}
	if pf.HasLatex {
		detected = append(detected, "LaTeX/TeX")
	}
	if pf.HasOpenCode {
		detected = append(detected, "OpenCode")
	}
	return detected
}

func inspectFile(name, nameLower string, features *ProjectFeatures) {
	if name == "go.mod" || name == "go.work" || strings.HasSuffix(nameLower, ".go") {
		features.HasGo = true
	}
	if name == "pyproject.toml" || name == "requirements.txt" || name == "Pipfile" ||
		name == "uv.lock" || name == "setup.py" || strings.HasSuffix(nameLower, ".py") {
		features.HasPython = true
	}
	if name == "Cargo.toml" || name == "Cargo.lock" || strings.HasSuffix(nameLower, ".rs") {
		features.HasRust = true
	}
	if name == "package.json" || name == "pnpm-lock.yaml" || name == "yarn.lock" ||
		name == "package-lock.json" || name == "bun.lockb" ||
		strings.HasSuffix(nameLower, ".js") || strings.HasSuffix(nameLower, ".ts") {
		features.HasNode = true
	}
	if name == "latexmkrc" || name == ".latexmkrc" || name == "Tectonic.toml" ||
		strings.HasSuffix(nameLower, ".tex") || strings.HasSuffix(nameLower, ".sty") ||
		strings.HasSuffix(nameLower, ".cls") || strings.HasSuffix(nameLower, ".dtx") ||
		strings.HasSuffix(nameLower, ".bib") || strings.HasSuffix(nameLower, ".ltx") {
		features.HasLatex = true
	}
	if name == "opencode.json" {
		features.HasOpenCode = true
	}
}

// DetectFeatures inspects the specified directory and detects project characteristics.
func DetectFeatures(dir string) (ProjectFeatures, error) {
	features := ProjectFeatures{EnableSSH: true}
	evidence, err := detect.Scan(dir)
	if err != nil {
		return features, err
	}
	for _, entry := range evidence.Entries {
		name := filepath.Base(entry.Path)
		if entry.Directory {
			if name == ".open-mem" || name == ".opencode" {
				features.HasOpenCode = true
			}
			continue
		}
		inspectFile(name, strings.ToLower(name), &features)
	}
	return features, nil
}
