package config

import (
	"os"
	"path/filepath"

	"bws/internal/util"
)

// WorkspaceCandidate is one directory on the upward search path that either
// holds a .bws configuration or is the current directory.
type WorkspaceCandidate struct {
	// Dir is the directory that (may) contain .bws.
	Dir string
	// ConfigPath is the concrete config file that applies, if present.
	ConfigPath string
	// Exists reports whether a .bws configuration was found in Dir.
	Exists bool
	// Target marks the single candidate a local write would use.
	Target bool
}

// WorkspaceCandidates returns the chain of .bws directories from startDir up to
// the same stop boundary FindWorkspaceRoot uses (home, ~/bin, or /), ordered
// nearest-first, with exactly one entry marked Target. It never diverges from
// read resolution: the Target is the nearest existing configuration, or
// startDir itself when none exists.
func WorkspaceCandidates(startDir string) []WorkspaceCandidate {
	home := util.HomeDir()
	homeReal, _ := filepath.EvalSymlinks(home)

	var chain []WorkspaceCandidate
	dir := filepath.Clean(startDir)
	for {
		dirReal, _ := filepath.EvalSymlinks(dir)
		binReal, _ := filepath.EvalSymlinks(filepath.Join(home, "bin"))
		if dir == "/" || dir == "." || dirReal == homeReal || dir == home || (binReal != "" && dirReal == binReal) {
			break
		}
		c := WorkspaceCandidate{Dir: dir}
		for _, p := range configCandidatesFor(dir) {
			if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
				c.ConfigPath = p
				c.Exists = true
				break
			}
		}
		chain = append(chain, c)
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	if len(chain) == 0 {
		chain = append(chain, WorkspaceCandidate{Dir: filepath.Clean(startDir)})
	}

	// Nearest-first: the first existing config wins; otherwise the nearest
	// directory (cwd) is the target.
	for i := range chain {
		if chain[i].Exists {
			chain[i].Target = true
			return chain
		}
	}
	chain[0].Target = true
	return chain
}

// configCandidatesFor lists the config filenames honored in a directory, in the
// same order and spelling as FindWorkspaceRoot.
func configCandidatesFor(dir string) []string {
	return []string{
		filepath.Join(dir, ".bws", "config.jsonc"),
		filepath.Join(dir, ".bws", "config.json"),
		filepath.Join(dir, ".bws.jsonc"),
	}
}
