package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bws/internal/config"
	"bws/internal/util"
)

func isPathInside(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel))
}

func tokenizePath(p string) string {
	homeDir := util.HomeDir()
	if homeDir == "" {
		return p
	}
	if p == homeDir {
		return config.HomeToken
	}
	if strings.HasPrefix(p, homeDir+"/") {
		return config.HomeToken + strings.TrimPrefix(p, homeDir)
	}
	return p
}

func resolveHostCheckPath(hostPath string, global bool) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}

	hostExpanded := utilExpandHome(hostPath)
	if filepath.IsAbs(hostExpanded) {
		return filepath.Clean(hostExpanded), nil
	}
	if global {
		return "", fmt.Errorf("global bind mount host path must be absolute")
	}
	return filepath.Clean(filepath.Join(cwd, hostExpanded)), nil
}

func evaluateSymlink(hostPath, checkPath string) (string, bool, error) {
	fi, err := os.Lstat(checkPath)
	if err != nil {
		return "", false, fmt.Errorf("host path '%s' does not exist (%w)", hostPath, err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		return hostPath, false, nil
	}

	target, err := filepath.EvalSymlinks(checkPath)
	if err != nil {
		return "", false, fmt.Errorf("symlink '%s' is dangling: target does not exist (%w)", hostPath, err)
	}
	if _, err := os.Stat(target); err != nil {
		return "", false, fmt.Errorf("symlink '%s' is dangling: target '%s' does not exist (%w)", hostPath, target, err)
	}

	cwd, _ := os.Getwd()
	evalCwd, _ := filepath.EvalSymlinks(cwd)
	if isPathInside(cwd, target) || (evalCwd != "" && isPathInside(evalCwd, target)) {
		fmt.Printf("Note: '%s' resolves to '%s' which is already accessible inside the workspace.\n", hostPath, target)
		return "", true, nil
	}

	tokenizedTarget := tokenizePath(target)
	fmt.Printf("Resolved symlink %s -> %s\n", hostPath, tokenizedTarget)
	return tokenizedTarget, false, nil
}

func runBindAdd(hostPath, sandboxPath string, rw, global, local bool) error {
	checkPath, err := resolveHostCheckPath(hostPath, global)
	if err != nil {
		return err
	}
	resolvedHost, skip, err := evaluateSymlink(hostPath, checkPath)
	if err != nil {
		return err
	}
	if skip {
		return nil
	}

	if !global && !local {
		local = true
	}
	targetPath := configFilePath(global)
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		if err := config.CreateDefault(targetPath); err != nil {
			return fmt.Errorf("creating default config: %w", err)
		}
	}

	key := "binds_ro"
	modeLabel := "read-only"
	if rw {
		key = "binds_rw"
		modeLabel = "read-write"
	}

	entry := fmt.Sprintf("%q", resolvedHost)
	if sandboxPath != "" && sandboxPath != resolvedHost {
		entry = fmt.Sprintf("[%q, %q]", resolvedHost, sandboxPath)
	}

	if err := config.AddBindArrayElement(targetPath, key, entry); err != nil {
		return err
	}

	label := "global"
	if local {
		label = "local"
	}
	fmt.Printf("Added %s bind mount '%s'", modeLabel, resolvedHost)
	if sandboxPath != "" && sandboxPath != resolvedHost {
		fmt.Printf(" -> '%s'", sandboxPath)
	}
	fmt.Printf(" to %s configuration (%s).\n", label, targetPath)
	return nil
}

func HandleBindAdd(hostPath, sandboxPath string, rw, global, local bool) {
	if err := runBindAdd(hostPath, sandboxPath, rw, global, local); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func HandleMountAdd(hostPath, sandboxPath string, rw, global, local bool) {
	HandleBindAdd(hostPath, sandboxPath, rw, global, local)
}

func runBindDel(hostPath string, global, local bool) error {
	if !global && !local {
		local = true
	}
	targetPath := configFilePath(global)

	candidates := []string{hostPath, tokenizePath(utilExpandHome(hostPath))}
	if checkPath, err := resolveHostCheckPath(hostPath, global); err == nil {
		if fi, err := os.Lstat(checkPath); err == nil && fi.Mode()&os.ModeSymlink != 0 {
			if target, err := filepath.EvalSymlinks(checkPath); err == nil {
				candidates = append(candidates, tokenizePath(target))
			}
		}
	}

	found := false
	var matchedPath string
	for _, cand := range candidates {
		for _, key := range []string{"binds_rw", "binds_ro"} {
			f, err := config.RemoveBindElement(targetPath, key, cand)
			if err != nil || f {
				found = found || f
			}
		}
		if found {
			matchedPath = cand
			break
		}
	}

	if !found {
		return fmt.Errorf("bind mount '%s' not found in configuration", hostPath)
	}

	label := "global"
	if local {
		label = "local"
	}
	fmt.Printf("Removed bind mount '%s' from %s configuration (%s).\n", matchedPath, label, targetPath)
	return nil
}

func HandleBindDel(hostPath string, global, local bool) {
	if err := runBindDel(hostPath, global, local); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func HandleMountDel(hostPath string, global, local bool) {
	HandleBindDel(hostPath, global, local)
}

func HandleMountList() {
	HandleBindList()
}

func HandleBindList() {
	globalPath := config.GlobalPath()
	localPath := config.LocalPath()
	printed := false

	for _, pair := range []struct {
		path  string
		label string
	}{{globalPath, "Global"}, {localPath, "Local"}} {
		if _, err := os.Stat(pair.path); os.IsNotExist(err) {
			continue
		}
		cfg, err := config.LoadFile(pair.path)
		if err != nil {
			continue
		}

		if len(cfg.BindsRW) == 0 && len(cfg.BindsRO) == 0 {
			continue
		}

		fmt.Printf("%s bind mounts (%s):\n", pair.label, pair.path)
		if len(cfg.BindsRW) > 0 {
			fmt.Println("  Read-Write:")
			for _, b := range cfg.BindsRW {
				if b.Sandbox != "" && b.Sandbox != b.Host {
					fmt.Printf("    %s -> %s\n", b.Host, b.Sandbox)
				} else {
					fmt.Printf("    %s\n", b.Host)
				}
			}
		}
		if len(cfg.BindsRO) > 0 {
			fmt.Println("  Read-Only:")
			for _, b := range cfg.BindsRO {
				if b.Sandbox != "" && b.Sandbox != b.Host {
					fmt.Printf("    %s -> %s\n", b.Host, b.Sandbox)
				} else {
					fmt.Printf("    %s\n", b.Host)
				}
			}
		}
		printed = true
	}
	if !printed {
		fmt.Println("No bind mounts configured.")
	}
}

func utilExpandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return home + path[1:]
	}
	return path
}
