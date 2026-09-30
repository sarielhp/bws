package config

import (
	"path/filepath"
	"strings"

	"bws/internal/util"
)

// systemRoots are host trees a local configuration may never bind read-write.
var systemRoots = []string{"/usr", "/bin", "/sbin", "/lib", "/lib32", "/lib64", "/libx32", "/etc"}

// RestrictRW drops non-global read-write binds (local config or profiles) that
// would make writable a host path the global configuration binds read-only as
// a whole, or any path under a system root. Subpaths of a global read-only
// directory stay allowed so a project can open a narrow writable hole. It
// returns the kept binds and a description of each rejected one.
func RestrictRW(globalRO, localRW []BindEntry) ([]BindEntry, []string) {
	readOnly := make(map[string]bool, len(globalRO))
	for _, b := range globalRO {
		if p := canonicalHostPath(b.Host); p != "" {
			readOnly[p] = true
		}
	}
	var kept []BindEntry
	var rejected []string
	for _, b := range localRW {
		p := canonicalHostPath(b.Host)
		switch {
		case p != "" && readOnly[p]:
			rejected = append(rejected, b.Host+" (read-only in global config)")
		case p != "" && underSystemRoot(p):
			rejected = append(rejected, b.Host+" (system directory)")
		default:
			kept = append(kept, b)
		}
	}
	return kept, rejected
}

// canonicalHostPath expands ~ and @@HOME@@, cleans the path and resolves
// symlinks when it exists. Relative paths return "" because they resolve
// against the workspace and cannot name a global or system path.
func canonicalHostPath(p string) string {
	p = strings.ReplaceAll(p, HomeToken, util.HomeDir())
	p = util.ExpandHome(p)
	if !filepath.IsAbs(p) {
		return ""
	}
	p = filepath.Clean(p)
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return resolved
	}
	return p
}

// underSystemRoot also rejects "/" itself, which would expose every root.
func underSystemRoot(p string) bool {
	if p == "/" {
		return true
	}
	for _, root := range systemRoots {
		for _, r := range []string{root, canonicalHostPath(root)} {
			if p == r || strings.HasPrefix(p, r+"/") {
				return true
			}
		}
	}
	return false
}
