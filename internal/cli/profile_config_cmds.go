package cli

import (
	"fmt"
	"os"

	"bws/internal/config"
	"bws/internal/profile"
)

// HandleProfileAdd adds one or more profile names to the profiles array in the config file.
func HandleProfileAdd(names []string, global, local bool) {
	if !global && !local {
		local = true
	}
	targetPath := configFilePath(global)

	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		config.CreateDefault(targetPath)
	}

	cfg, err := config.LoadFile(targetPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	cwd, _ := os.Getwd()
	reg, _ := profile.LoadRegistry(cwd)

	existing := make(map[string]bool)
	for _, p := range cfg.Profiles {
		existing[p] = true
	}

	var added []string
	for _, name := range names {
		if reg != nil {
			if _, ok := reg[name]; !ok {
				fmt.Fprintf(os.Stderr, "Warning: Profile %s is not found in known profiles registry.\n", ColorProfile(name))
			}
		}
		if existing[name] {
			fmt.Printf("Profile %s is already enabled in %s configuration.\n", ColorProfile(name), targetPath)
			continue
		}
		cfg.Profiles = append(cfg.Profiles, name)
		existing[name] = true
		added = append(added, name)
	}

	if len(added) == 0 {
		return
	}

	if err := config.SetArrayValue(targetPath, "profiles", cfg.Profiles); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}

	label := "global"
	if !global {
		label = "local"
	}
	for _, name := range added {
		fmt.Printf("Added profile %s to %s sandbox configuration (%s).\n", ColorProfile(name), label, targetPath)
	}
}

// HandleProfileDel removes one or more profile names from the profiles array in the config file.
func HandleProfileDel(names []string, global, local bool) {
	if !global && !local {
		local = true
	}
	targetPath := configFilePath(global)

	cfg, err := config.LoadFile(targetPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	toRemove := make(map[string]bool)
	for _, name := range names {
		toRemove[name] = true
	}

	var newProfiles []string
	var removed []string
	for _, p := range cfg.Profiles {
		if toRemove[p] {
			removed = append(removed, p)
		} else {
			newProfiles = append(newProfiles, p)
		}
	}

	label := "global"
	if !global {
		label = "local"
	}

	for _, name := range names {
		found := false
		for _, r := range removed {
			if r == name {
				found = true
				break
			}
		}
		if !found {
			fmt.Fprintf(os.Stderr, "Profile %s not found in %s configuration (%s).\n", ColorProfile(name), label, targetPath)
		}
	}

	if len(removed) == 0 {
		os.Exit(1)
	}

	if err := config.SetArrayValue(targetPath, "profiles", newProfiles); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}

	for _, name := range removed {
		fmt.Printf("Removed profile %s from %s sandbox configuration (%s).\n", ColorProfile(name), label, targetPath)
	}
}
