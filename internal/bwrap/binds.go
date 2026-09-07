package bwrap

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"bws/internal/config"
	"bws/internal/util"
)

type bindItem struct {
	host string
	dest string
	ro   bool
}

func resolveBindPath(p, homeDir, currentDir string) string {
	if p == "" {
		return ""
	}
	p = strings.ReplaceAll(p, config.HomeToken, homeDir)
	p = util.ExpandHome(p)
	if !filepath.IsAbs(p) && currentDir != "" {
		p = filepath.Clean(filepath.Join(currentDir, p))
	}
	return filepath.Clean(p)
}

func pathDepth(p string) int {
	cleaned := filepath.Clean(p)
	trimmed := strings.TrimRight(cleaned, "/")
	if trimmed == "" {
		return 0
	}
	return strings.Count(trimmed, "/")
}

func collectBinds(entries []config.BindEntry, homeDir, currentDir, sandboxDir string, ro, verbose bool) []bindItem {
	var items []bindItem
	for _, b := range entries {
		dest := b.Sandbox
		if dest == "" {
			dest = b.Host
		}
		dest = resolveBindPath(dest, homeDir, currentDir)
		host := resolveBindPath(b.Host, homeDir, currentDir)
		if !ro && sandboxDir != "" && host == sandboxDir && dest == homeDir {
			continue
		}
		if _, err := os.Stat(host); err == nil {
			items = append(items, bindItem{host: host, dest: dest, ro: ro})
		} else if verbose {
			tag := "RW"
			if ro {
				tag = "RO"
			}
			fmt.Fprintf(os.Stderr, "[verbose]   (skipping %s bind: %s does not exist)\n", tag, host)
		}
	}
	return items
}

func buildBinds(cfg *config.Config, sandboxDir, homeDir, currentDir string, verbose bool) []string {
	var allBinds []bindItem
	allBinds = append(allBinds, collectBinds(cfg.BindsRO, homeDir, currentDir, sandboxDir, true, verbose)...)
	allBinds = append(allBinds, collectBinds(cfg.BindsRW, homeDir, currentDir, sandboxDir, false, verbose)...)

	// Sort so parent directories are bound before child subdirectories (depth parent < depth child).
	// When depth is equal, RO comes before RW so child RW sub-mounts can override parent RO mounts.
	sort.SliceStable(allBinds, func(i, j int) bool {
		depthI := pathDepth(allBinds[i].dest)
		depthJ := pathDepth(allBinds[j].dest)
		if depthI != depthJ {
			return depthI < depthJ
		}
		if allBinds[i].ro != allBinds[j].ro {
			return allBinds[i].ro // ro before rw
		}
		if allBinds[i].dest != allBinds[j].dest {
			return allBinds[i].dest < allBinds[j].dest
		}
		return allBinds[i].host < allBinds[j].host
	})

	var args []string
	for _, b := range allBinds {
		flag := "--bind"
		if b.ro {
			flag = "--ro-bind"
		}
		args = append(args, flag, b.host, b.dest)
		if verbose {
			fmt.Fprintf(os.Stderr, "[verbose]   %s %s %s\n", flag, b.host, b.dest)
		}
	}
	return args
}
