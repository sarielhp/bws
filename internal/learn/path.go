package learn

import (
	"os/exec"
	"path/filepath"
	"strings"
)

// DefaultSystemPaths returns the standard base PATH directories for comparison.
func DefaultSystemPaths(homeDir string) []string {
	paths := []string{
		"/usr/bin",
		"/bin",
		"/sbin",
		"/usr/local/bin",
		"/usr/local/sbin",
		"/usr/games",
		"/usr/local/games",
		"/snap/bin",
	}

	if homeDir != "" {
		paths = append(paths,
			"~/.local/bin",
			"~/bin",
			"@@HOME@@/.local/bin",
			"@@HOME@@/bin",
		)
	}

	return paths
}

// CanonicalPath expands ~, @@HOME@@, and symlinks/relative tokens into a clean absolute path for comparison.
func CanonicalPath(p, homeDir string) string {
	clean := strings.TrimSpace(p)
	if clean == "" {
		return ""
	}

	if homeDir != "" {
		cleanHome := filepath.Clean(homeDir)
		if clean == "~" || clean == "@@HOME@@" {
			clean = cleanHome
		} else if strings.HasPrefix(clean, "~/") {
			clean = filepath.Join(cleanHome, clean[2:])
		} else if strings.HasPrefix(clean, "@@HOME@@/") {
			clean = filepath.Join(cleanHome, strings.TrimPrefix(clean, "@@HOME@@/"))
		}
	}

	if !filepath.IsAbs(clean) {
		if abs, err := filepath.Abs(clean); err == nil {
			clean = abs
		}
	}

	return filepath.Clean(clean)
}

// IsPathInList checks if targetDir is present in pathList by comparing canonical paths.
func IsPathInList(targetDir string, pathList []string, homeDir string) bool {
	canonTarget := CanonicalPath(targetDir, homeDir)
	if canonTarget == "" {
		return false
	}

	for _, item := range pathList {
		canonItem := CanonicalPath(item, homeDir)
		if canonItem == canonTarget {
			return true
		}
	}

	return false
}

// IsPathCovered checks if targetDir is in existing config paths or base system defaults.
func IsPathCovered(targetDir string, configPaths []string, homeDir string) bool {
	if IsPathInList(targetDir, configPaths, homeDir) {
		return true
	}
	return IsPathInList(targetDir, DefaultSystemPaths(homeDir), homeDir)
}

// ResolveBinaryDir locates the directory containing the command executable and returns its normalized path.
func ResolveBinaryDir(cmdName, workDir, homeDir string) (string, error) {
	cleanCmd := strings.TrimSpace(cmdName)
	if cleanCmd == "" {
		return "", nil
	}

	var absPath string
	if strings.Contains(cleanCmd, "/") || strings.HasPrefix(cleanCmd, ".") || strings.HasPrefix(cleanCmd, "~") {
		expanded := cleanCmd
		if strings.HasPrefix(expanded, "~/") && homeDir != "" {
			expanded = filepath.Join(homeDir, expanded[2:])
		} else if expanded == "~" && homeDir != "" {
			expanded = homeDir
		}

		if !filepath.IsAbs(expanded) && workDir != "" {
			expanded = filepath.Join(workDir, expanded)
		}

		abs, err := filepath.Abs(expanded)
		if err != nil {
			return "", err
		}
		absPath = abs
	} else {
		looked, err := exec.LookPath(cleanCmd)
		if err != nil {
			return "", err
		}
		abs, err := filepath.Abs(looked)
		if err != nil {
			abs = looked
		}
		absPath = abs
	}

	dir := filepath.Dir(absPath)
	norm := NormalizePath(dir, homeDir)
	return norm, nil
}
