package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"bws/internal/config"
	"bws/internal/profile"
)

func ensureProfileInRegistry(name string, global, local, create, force bool, cwd string, reg map[string]*profile.Profile) error {
	p := reg[name]
	if p == nil {
		if !create {
			return fmt.Errorf("profile %s not found in catalog.\n  • Run 'bws profile list' to see available profiles.\n  • Run 'bws add -c %s' to synthesize and add it.", ColorProfile(name), name)
		}

		// Verification that the tool is installed in the system before synthesizing and adding
		if _, err := profile.VerifyToolInstalled(name, nil); err != nil && !force {
			return fmt.Errorf("cannot add profile %q: tool is not installed on host system ($PATH).\nInstall %q first, or use -f/--force to add anyway", name, name)
		}

		pNew, info, err := profile.GenerateProfileDetailed(name, reg)
		if err != nil {
			return fmt.Errorf("synthesizing profile %q: %w", name, err)
		}
		if !info.HomebrewFormula && !info.FirejailProfile && !force {
			return fmt.Errorf("cannot synthesize profile for %q: not found in Homebrew formulae or Firejail profiles", name)
		}

		PrintSynthesisSource(name, info)

		var targetDir string
		if local && !global {
			targetDir = profile.LocalProfilesDir(cwd)
		} else {
			targetDir = profile.GlobalProfilesDir()
		}

		targetPath := filepath.Join(targetDir, name+".json")
		if err := profile.SaveProfile(pNew, targetPath); err != nil {
			return fmt.Errorf("saving profile %q: %w", name, err)
		}

		fmt.Printf("✓ Saved synthesized profile to: %s\n", targetPath)
		p = pNew
		if reg != nil {
			reg[name] = p
			for _, alias := range p.Aliases {
				reg[alias] = p
			}
		}
	}

	// Verification that the tool is installed in the system before adding the profile
	if profile.IsToolProfile(p) {
		if _, err := profile.VerifyToolInstalled(name, p); err != nil {
			if !force {
				return fmt.Errorf("cannot add profile %q: tool is not installed on host system ($PATH).\nInstall %q first, or use -f/--force to add anyway", name, name)
			}
			fmt.Printf("Warning: tool %q is not installed on host ($PATH); adding profile anyway (--force)\n", name)
		}
	}
	return nil
}

// HandleProfileAdd adds one or more profile names to the profiles array in the config file.
func HandleProfileAdd(names []string, global, local, create, force bool) error {
	if !global && !local {
		local = true
	}
	targetPath := configFilePath(global)

	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		if err := config.CreateDefault(targetPath); err != nil {
			return fmt.Errorf("creating default config: %w", err)
		}
	}

	cfg, err := config.LoadFile(targetPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	cwd, _ := os.Getwd()
	reg, err := profile.LoadRegistry(cwd)
	if err != nil {
		return fmt.Errorf("loading profile registry: %w", err)
	}

	for _, name := range names {
		if err := ensureProfileInRegistry(name, global, local, create, force, cwd, reg); err != nil {
			return err
		}
	}

	existing := make(map[string]bool)
	for _, p := range cfg.Profiles {
		existing[p] = true
	}

	var added []string
	var refreshed []string
	for _, name := range names {
		if existing[name] {
			refreshed = append(refreshed, name)
			continue
		}
		cfg.Profiles = append(cfg.Profiles, name)
		existing[name] = true
		added = append(added, name)
	}

	if len(added) > 0 {
		if err := config.SetArrayValue(targetPath, "profiles", cfg.Profiles); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
	}

	label := "global"
	if !global {
		label = "local"
	}
	for _, name := range added {
		fmt.Printf("Added profile %s to %s sandbox configuration (%s).\n", ColorProfile(name), label, formatConfigDisplay(targetPath, global))
	}
	for _, name := range refreshed {
		fmt.Printf("Profile %s is active in %s sandbox configuration (%s).\n", ColorProfile(name), label, formatConfigDisplay(targetPath, global))
	}
	return nil
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
			fmt.Fprintf(os.Stderr, "Profile %s not found in %s configuration (%s).\n", ColorProfile(name), label, formatConfigDisplay(targetPath, global))
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
		fmt.Printf("Removed profile %s from %s sandbox configuration (%s).\n", ColorProfile(name), label, formatConfigDisplay(targetPath, global))
	}
}
