package config

import (
	"fmt"
	"path/filepath"

	"bws/internal/util"
)

// ValidateWorkspace checks the directory actually exposed by automatic mounts.
func ValidateWorkspace(dir string, limit int, force bool) error {
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return fmt.Errorf("resolving workspace: %w", err)
	}
	home, err := filepath.EvalSymlinks(util.HomeDir())
	if err != nil {
		return fmt.Errorf("resolving home: %w", err)
	}
	bin := filepath.Join(home, "bin")
	if resolved, err := filepath.EvalSymlinks(bin); err == nil {
		bin = resolved
	}
	for _, blocked := range []struct{ path, label string }{{"/", "/"}, {home, "~/"}, {bin, "~/bin/"}} {
		if real == blocked.path {
			return fmt.Errorf("running the sandbox from %s is blocked", blocked.label)
		}
	}
	root, _ := FindWorkspaceRoot(dir)
	if limit <= 0 {
		limit = 1000
	}
	if !force {
		if count := util.CountFiles(root, limit); count > limit {
			return fmt.Errorf("workspace directory contains more than %d files (found %d); use -f to override", limit, count)
		}
	}
	return nil
}
