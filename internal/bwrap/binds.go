package bwrap

import (
	"bws/internal/config"
	"bws/internal/util"
	"fmt"
	"os"
	"sort"
	"strings"
)

func buildBinds(cfg *config.Config, sandboxDir, homeDir string, verbose bool) []string {
	var args []string
	type bindItem struct {
		host string
		dest string
		ro   bool
	}
	var allBinds []bindItem

	for _, b := range cfg.BindsRO {
		dest := b.Sandbox
		if dest == "" {
			dest = b.Host
		}
		dest = strings.ReplaceAll(dest, config.HomeToken, homeDir)
		dest = util.ExpandHome(dest)
		host := util.ExpandHome(b.Host)
		if _, err := os.Stat(host); err == nil {
			allBinds = append(allBinds, bindItem{host: host, dest: dest, ro: true})
		} else if verbose {
			fmt.Fprintf(os.Stderr, "[verbose]   (skipping RO bind: %s does not exist)\n", host)
		}
	}

	for _, b := range cfg.BindsRW {
		dest := b.Sandbox
		if dest == "" {
			dest = b.Host
		}
		dest = strings.ReplaceAll(dest, config.HomeToken, homeDir)
		dest = util.ExpandHome(dest)
		host := util.ExpandHome(b.Host)
		if sandboxDir != "" && host == sandboxDir && dest == homeDir {
			continue
		}
		if _, err := os.Stat(host); err == nil {
			allBinds = append(allBinds, bindItem{host: host, dest: dest, ro: false})
		} else if verbose {
			fmt.Fprintf(os.Stderr, "[verbose]   (skipping RW bind: %s does not exist)\n", host)
		}
	}

	// Sort so parent directories are bound before child subdirectories.
	// When depth is equal, RO comes before RW so child RW sub-mounts can override parent RO mounts.
	sort.SliceStable(allBinds, func(i, j int) bool {
		depthI := strings.Count(strings.TrimRight(allBinds[i].dest, "/"), "/")
		depthJ := strings.Count(strings.TrimRight(allBinds[j].dest, "/"), "/")
		if depthI != depthJ {
			return depthI < depthJ
		}
		if len(allBinds[i].dest) != len(allBinds[j].dest) {
			return len(allBinds[i].dest) < len(allBinds[j].dest)
		}
		if allBinds[i].ro != allBinds[j].ro {
			return allBinds[i].ro // ro before rw
		}
		return allBinds[i].dest < allBinds[j].dest
	})

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
