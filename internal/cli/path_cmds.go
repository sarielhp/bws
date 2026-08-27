package cli

import (
	"fmt"
	"os"
	"strings"

	"bw/internal/config"
)

// HandlePathAdd adds a directory to the PATH array in the config
func HandlePathAdd(dir string, global, local bool) {
	ValidateGL(global, local)
	targetPath := configFilePath(global)

	if !strings.HasPrefix(dir, "/") && !strings.HasPrefix(dir, "~/") {
		fmt.Fprintf(os.Stderr, "Error: Directory path must be absolute or start with ~/.\n")
		os.Exit(1)
	}

	cfg, err := config.LoadFile(targetPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Check for duplicates
	for _, p := range cfg.Path {
		if p == dir {
			fmt.Printf("Directory '%s' is already in the PATH.\n", dir)
			os.Exit(0)
		}
	}

	// Add to the path array
	cfg.Path = append(cfg.Path, dir)
	if err := config.SetArrayValue(targetPath, "path", cfg.Path); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}

	label := "global"
	if !global {
		label = "local"
	}
	fmt.Printf("Added '%s' to %s PATH configuration (%s).\n", dir, label, targetPath)
}

// HandlePathDel removes a directory from the PATH array in the config
func HandlePathDel(dir string, global, local bool) {
	ValidateGL(global, local)
	targetPath := configFilePath(global)

	cfg, err := config.LoadFile(targetPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Find and remove the directory
	found := false
	newPath := make([]string, 0, len(cfg.Path))
	for _, p := range cfg.Path {
		if p == dir {
			found = true
		} else {
			newPath = append(newPath, p)
		}
	}

	if !found {
		fmt.Fprintf(os.Stderr, "Directory '%s' not found in PATH configuration.\n", dir)
		os.Exit(1)
	}

	// Save the updated config
	if err := config.SetArrayValue(targetPath, "path", newPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}

	label := "global"
	if !global {
		label = "local"
	}
	fmt.Printf("Removed '%s' from %s PATH configuration.\n", dir, label)
}

// HandlePathList lists combined paths from global and local configs with [g] and [l] indicators
func HandlePathList() {
	globalPath := config.GlobalPath()
	localPath := config.LocalPath()

	var globalPaths []string
	if _, err := os.Stat(globalPath); err == nil {
		if cfg, err := config.LoadFile(globalPath); err == nil {
			globalPaths = cfg.Path
		}
	}

	var localPaths []string
	if _, err := os.Stat(localPath); err == nil {
		if cfg, err := config.LoadFile(localPath); err == nil {
			localPaths = cfg.Path
		}
	}

	if len(globalPaths) == 0 && len(localPaths) == 0 {
		fmt.Println("No PATH entries configured.")
		return
	}

	seen := make(map[string]bool)
	var combined []struct {
		path   string
		source string
	}

	for _, p := range globalPaths {
		if !seen[p] {
			seen[p] = true
			combined = append(combined, struct {
				path   string
				source string
			}{path: p, source: "[g]"})
		}
	}

	for _, p := range localPaths {
		if !seen[p] {
			seen[p] = true
			combined = append(combined, struct {
				path   string
				source string
			}{path: p, source: "[l]"})
		} else {
			// Update source indicator if present in both or local overrides
			for i, item := range combined {
				if item.path == p {
					combined[i].source = "[g,l]"
					break
				}
			}
		}
	}

	for _, item := range combined {
		fmt.Printf("  %s %s\n", item.source, item.path)
	}
}
