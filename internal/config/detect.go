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

		// Go detection
		if name == "go.mod" || name == "go.work" || strings.HasSuffix(nameLower, ".go") {
			features.HasGo = true
		}

		// Python detection
		if name == "pyproject.toml" || name == "requirements.txt" || name == "Pipfile" ||
			name == "uv.lock" || name == "setup.py" || strings.HasSuffix(nameLower, ".py") {
			features.HasPython = true
		}

		// Rust detection
		if name == "Cargo.toml" || name == "Cargo.lock" || strings.HasSuffix(nameLower, ".rs") {
			features.HasRust = true
		}

		// Node detection
		if name == "package.json" || name == "pnpm-lock.yaml" || name == "yarn.lock" ||
			name == "package-lock.json" || name == "bun.lockb" ||
			strings.HasSuffix(nameLower, ".js") || strings.HasSuffix(nameLower, ".ts") {
			features.HasNode = true
		}

		// LaTeX / TeX detection
		if name == "latexmkrc" || name == ".latexmkrc" || name == "Tectonic.toml" ||
			strings.HasSuffix(nameLower, ".tex") || strings.HasSuffix(nameLower, ".sty") ||
			strings.HasSuffix(nameLower, ".cls") || strings.HasSuffix(nameLower, ".dtx") ||
			strings.HasSuffix(nameLower, ".bib") || strings.HasSuffix(nameLower, ".ltx") {
			features.HasLatex = true
		}

		// OpenCode detection
		if name == "opencode.json" {
			features.HasOpenCode = true
		}

		return nil
	})

	if walkErr != nil {
		return features, walkErr
	}

	return features, nil
}
