package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bws/internal/config"
	"bws/internal/util"
)

// HandleBinAdd exposes a host executable inside the sandbox as a read-only bind mount.
func HandleBinAdd(hostPath string, global, local bool) {
	if !global && !local {
		local = true
	}
	targetPath := configFilePath(global)
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		config.CreateDefault(targetPath)
	}

	expanded := utilExpandHome(hostPath)
	if !strings.HasPrefix(expanded, "/") {
		abs, err := filepath.Abs(expanded)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Invalid path '%s': %v\n", hostPath, err)
			os.Exit(1)
		}
		expanded = abs
	}

	fi, err := os.Stat(expanded)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: File '%s' does not exist.\n", expanded)
		os.Exit(1)
	}
	if fi.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: '%s' is a directory. Use 'bws mount add' to mount directories.\n", expanded)
		os.Exit(1)
	}
	if fi.Mode().Perm()&0111 == 0 {
		fmt.Fprintf(os.Stderr, "Warning: '%s' does not have execute permissions on host (consider running 'chmod +x %s').\n", expanded, expanded)
	}

	homeDir, _ := os.UserHomeDir()
	binDir := filepath.Join(homeDir, "bin")
	localBinDir := filepath.Join(homeDir, ".local", "bin")
	baseName := filepath.Base(expanded)

	var sandboxPath string
	if strings.HasPrefix(expanded, binDir+"/") || strings.HasPrefix(expanded, localBinDir+"/") {
		sandboxPath = expanded
	} else {
		sandboxPath = filepath.Join(binDir, baseName)
	}

	var entry string
	if expanded == sandboxPath {
		entry = fmt.Sprintf("%q", hostPath)
	} else {
		entry = fmt.Sprintf("[%q, %q]", hostPath, sandboxPath)
	}

	cfg, err := config.LoadFile(targetPath)
	if err == nil {
		for _, b := range cfg.BindsRO {
			if util.ExpandHome(b.Host) == expanded {
				label := "local"
				if global {
					label = "global"
				}
				fmt.Printf("Binary '%s' is already configured in %s configuration.\n", hostPath, label)
				return
			}
		}
	}

	if err := config.AddBindArrayElement(targetPath, "binds_ro", entry); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	label := "local"
	if global {
		label = "global"
	}
	fmt.Printf("Added executable '%s'", hostPath)
	if expanded != sandboxPath {
		fmt.Printf(" -> '%s'", sandboxPath)
	}
	fmt.Printf(" as read-only binary to %s configuration (%s).\n", label, targetPath)
}

// HandleBinDel removes an exposed binary from the configuration.
func HandleBinDel(nameOrPath string, global, local bool) {
	if !global && !local {
		local = true
	}
	targetPath := configFilePath(global)
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Configuration file not found: %s\n", targetPath)
		os.Exit(1)
	}
	cfg, err := config.LoadFile(targetPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading config: %v\n", err)
		os.Exit(1)
	}

	var matchedHost string
	targetExpanded := util.ExpandHome(nameOrPath)
	for _, b := range cfg.BindsRO {
		hostExpanded := util.ExpandHome(b.Host)
		destExpanded := util.ExpandHome(b.Sandbox)
		if b.Host == nameOrPath || hostExpanded == targetExpanded ||
			filepath.Base(b.Host) == nameOrPath || filepath.Base(destExpanded) == nameOrPath {
			matchedHost = b.Host
			break
		}
	}

	if matchedHost == "" {
		fmt.Fprintf(os.Stderr, "Binary '%s' not found in configuration.\n", nameOrPath)
		os.Exit(1)
	}

	found, err := config.RemoveBindElement(targetPath, "binds_ro", matchedHost)
	if err != nil || !found {
		fmt.Fprintf(os.Stderr, "Failed to remove binary '%s' from configuration: %v\n", matchedHost, err)
		os.Exit(1)
	}

	label := "local"
	if global {
		label = "global"
	}
	fmt.Printf("Removed binary '%s' from %s configuration (%s).\n", matchedHost, label, targetPath)
}

// HandleBinList lists all exposed binaries in the configuration.
func HandleBinList() {
	globalPath := config.GlobalPath()
	localPath := config.LocalPath()
	printed := false

	homeDir, _ := os.UserHomeDir()
	binDir := filepath.Join(homeDir, "bin")

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

		var binaries []config.BindEntry
		for _, b := range cfg.BindsRO {
			dest := b.Sandbox
			if dest == "" {
				dest = b.Host
			}
			destExp := util.ExpandHome(dest)
			hostExp := util.ExpandHome(b.Host)
			fi, err := os.Stat(hostExp)
			if err != nil || fi.IsDir() {
				continue
			}
			isExec := fi.Mode().Perm()&0111 != 0
			inBin := strings.HasPrefix(destExp, binDir+"/") || strings.Contains(destExp, "/bin/")
			if isExec || inBin {
				binaries = append(binaries, b)
			}
		}

		if len(binaries) > 0 {
			fmt.Printf("%s binaries (%s):\n", pair.label, pair.path)
			for _, b := range binaries {
				if b.Sandbox != "" && b.Sandbox != b.Host {
					fmt.Printf("  %s -> %s\n", b.Host, b.Sandbox)
				} else {
					fmt.Printf("  %s\n", b.Host)
				}
			}
			printed = true
		}
	}

	if !printed {
		fmt.Println("No binaries configured.")
	}
}
