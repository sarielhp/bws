package cli

import (
	"fmt"
	"os"

	"bws/internal/config"
)

// HandleUndo restores the target configuration from its single-slot backup.
// The restored local file is deliberately left untrusted so it must be
// reviewed with 'bws config trust' before further use.
func HandleUndo(global, local bool) {
	if !global && !local {
		local = true
	}
	path := configFilePath(global)
	label := "local"
	if global {
		label = "global"
	}

	if !config.IsConfigFile(path) {
		fmt.Fprintf(os.Stderr, "Error: cannot undo %s\n", path)
		os.Exit(1)
	}
	if !config.HasBackup(path) {
		fmt.Fprintf(os.Stderr, "No backup available for %s configuration (%s).\n", label, config.DisplayPath(config.BackupPath(path)))
		os.Exit(1)
	}

	if err := config.RestoreBackup(path); err != nil {
		fmt.Fprintf(os.Stderr, "Error restoring config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Restored %s configuration from %s (state before the most recent write).\n", label, config.DisplayPath(config.BackupPath(path)))
	if !global {
		PrintWorkspaceInfo(findWorkspaceForPath(path))
		fmt.Println("This file is no longer trusted; run 'bws config trust' in its workspace after reviewing it.")
	}
}
