package profile

import (
	"os"
	"path/filepath"
	"strings"
)

func DetectProfiles(projectDir string, registry map[string]*Profile) []*Profile {
	var detected []*Profile
	seen := make(map[string]bool)

	// Collect files in project root (shallow check for fast detection)
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return nil
	}

	var rootFiles []string
	for _, e := range entries {
		rootFiles = append(rootFiles, e.Name())
	}

	dirBase := filepath.Base(projectDir)

	for _, p := range registry {
		if p.Detect == nil {
			continue
		}
		if seen[p.Name] {
			continue
		}

		matched := false

		// Check exact files
		for _, f := range p.Detect.Files {
			for _, rf := range rootFiles {
				if strings.EqualFold(f, rf) {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}

		// Check globs against root files
		if !matched && len(p.Detect.Globs) > 0 {
			for _, g := range p.Detect.Globs {
				for _, rf := range rootFiles {
					if matchGlob(g, rf) {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}
		}

		// Check directory name substrings
		if !matched && len(p.Detect.DirContains) > 0 {
			for _, sub := range p.Detect.DirContains {
				if strings.Contains(strings.ToLower(dirBase), strings.ToLower(sub)) {
					matched = true
					break
				}
			}
		}

		if matched {
			seen[p.Name] = true
			detected = append(detected, p)
		}
	}

	return detected
}

func matchGlob(pattern, name string) bool {
	matched, err := filepath.Match(pattern, name)
	if err != nil {
		return false
	}
	return matched
}
