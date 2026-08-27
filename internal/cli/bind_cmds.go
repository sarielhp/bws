package cli

import (
	"fmt"
	"os"
	"strings"

	"bw/internal/config"
)

func HandleBindAdd(hostPath, sandboxPath string, ro, global, local bool) {
	ValidateGL(global, local)
	targetPath := configFilePath(global)
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		config.CreateDefault(targetPath)
	}

	hostExpanded := utilExpandHome(hostPath)
	if !strings.HasPrefix(hostExpanded, "/") {
		fmt.Fprintf(os.Stderr, "Error: Host path must be absolute.\n")
		os.Exit(1)
	}

	key := "binds_rw"
	modeLabel := "read-write"
	if ro {
		key = "binds_ro"
		modeLabel = "read-only"
	}

	entry := fmt.Sprintf("%q", hostPath)
	if sandboxPath != "" && sandboxPath != hostPath {
		entry = fmt.Sprintf("[%q, %q]", hostPath, sandboxPath)
	}

	if err := config.AddBindArrayElement(targetPath, key, entry); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	label := "global"
	if local {
		label = "local"
	}
	fmt.Printf("Added %s bind mount '%s'", modeLabel, hostPath)
	if sandboxPath != "" && sandboxPath != hostPath {
		fmt.Printf(" -> '%s'", sandboxPath)
	}
	fmt.Printf(" to %s configuration (%s).\n", label, targetPath)
}

func HandleBindDel(hostPath string, global, local bool) {
	ValidateGL(global, local)
	targetPath := configFilePath(global)

	found := false
	for _, key := range []string{"binds_rw", "binds_ro"} {
		f, err := config.RemoveBindElement(targetPath, key, hostPath)
		if err != nil || f {
			found = found || f
		}
	}

	if !found {
		fmt.Fprintf(os.Stderr, "Bind mount '%s' not found in configuration.\n", hostPath)
		os.Exit(1)
	}

	label := "global"
	if local {
		label = "local"
	}
	fmt.Printf("Removed bind mount '%s' from %s configuration (%s).\n", hostPath, label, targetPath)
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
