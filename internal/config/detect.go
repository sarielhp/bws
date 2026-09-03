package config

import (
	"io/fs"
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
	features := ProjectFeatures{
		EnableSSH: true,
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return features, err
	}

	dirLower := strings.ToLower(filepath.Base(absDir))
	if strings.Contains(dirLower, "opencode") || strings.Contains(dirLower, "oc") {
		features.HasOpenCode = true
	}

	maxDepth := 3
	walkErr := filepath.WalkDir(absDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		rel, err := filepath.Rel(absDir, path)
		if err != nil {
			return nil
		}

		if d.IsDir() {
			name := d.Name()
			if rel != "." {
				// Skip hidden or heavy directories
				if strings.HasPrefix(name, ".") && name != ".open-mem" && name != ".opencode" {
					return filepath.SkipDir
				}
				if name == "node_modules" || name == "vendor" || name == "target" ||
					name == "dist" || name == "build" || name == "__pycache__" ||
					name == ".venv" || name == "venv" || name == ".sandbox" {
					return filepath.SkipDir
				}
			}

			if name == ".open-mem" || name == ".opencode" {
				features.HasOpenCode = true
			}

			depth := strings.Count(rel, string(filepath.Separator))
			if rel != "." && depth >= maxDepth {
				return filepath.SkipDir
			}
			return nil
		}

		name := d.Name()
		nameLower := strings.ToLower(name)
		inspectFile(name, nameLower, &features)
		return nil
	})

	if walkErr != nil {
		return features, walkErr
	}

	return features, nil
}
