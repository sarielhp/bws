package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bws/internal/config"
	"bws/internal/profile"
)

// HandleInit handles the `bws init` command.
func HandleInit(targetDir string, force, dryRun, noSSH, opencode bool, preset string, profiles []string) error {
	return HandleInitDev(targetDir, force, dryRun, noSSH, opencode, preset, profiles)
}

// HandleInitDev handles the `bws init-dev` command.
func HandleInitDev(targetDir string, force, dryRun, noSSH, opencode bool, preset string, profiles []string) error {
	if targetDir == "" {
		targetDir = "."
	}

	absDir, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("resolving target directory: %w", err)
	}

	fi, err := os.Stat(absDir)
	if err != nil {
		return fmt.Errorf("target directory %s: %w", absDir, err)
	}
	if !fi.IsDir() {
		return fmt.Errorf("target path is not a directory: %s", absDir)
	}

	features, err := config.DetectFeatures(absDir)
	if err != nil {
		return fmt.Errorf("detecting workspace features: %w", err)
	}

	activeProfiles, extraRW, extraRO, extraPath, extraEnv := resolveInitProfiles(absDir, profiles)

	opts := config.InitDevOptions{
		Features:     features,
		TargetDir:    absDir,
		Force:        force,
		DryRun:       dryRun,
		NoSSH:        noSSH,
		OpenCode:     opencode,
		Preset:       preset,
		Profiles:     activeProfiles,
		ExtraBindsRW: extraRW,
		ExtraBindsRO: extraRO,
		ExtraPath:    extraPath,
		ExtraEnv:     extraEnv,
	}

	jsonContent, err := config.GenerateDevConfigJSON(opts)
	if err != nil {
		return fmt.Errorf("generating dev configuration: %w", err)
	}

	if dryRun {
		fmt.Print(jsonContent)
		return nil
	}

	configPath := config.FindLocalPath(absDir)
	if err := writeConfigFile(configPath, jsonContent, force); err != nil {
		return err
	}

	printInitSummary(configPath, opts)
	return nil
}

func resolveInitProfiles(absDir string, profiles []string) ([]string, [][]string, [][]string, []string, map[string]string) {
	registry, _ := profile.LoadRegistry(absDir)
	detectedProfiles := profile.DetectProfiles(absDir, registry)
	activeProfileNames := make(map[string]bool)

	for _, p := range detectedProfiles {
		activeProfileNames[p.Name] = true
	}
	for _, pName := range profiles {
		clean := strings.TrimSpace(pName)
		if clean != "" {
			activeProfileNames[clean] = true
		}
	}

	var extraRW [][]string
	var extraRO [][]string
	var extraPath []string
	extraEnv := make(map[string]string)
	ctx := profile.DetectMatchContext()

	var finalActiveProfiles []string
	for pName := range activeProfileNames {
		finalActiveProfiles = append(finalActiveProfiles, pName)
		if resolved, err := profile.ResolveProfile(pName, registry, ctx); err == nil {
			extraRW = append(extraRW, resolved.BindsRW...)
			extraRO = append(extraRO, resolved.BindsRO...)
			extraPath = append(extraPath, resolved.Path...)
			for k, v := range resolved.Env {
				extraEnv[k] = v
			}
		}
	}
	return finalActiveProfiles, extraRW, extraRO, extraPath, extraEnv
}

func writeConfigFile(configPath, jsonContent string, force bool) error {
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", filepath.Dir(configPath), err)
	}

	if _, err := os.Stat(configPath); err == nil && !force {
		backupPath := configPath + ".bak"
		if err := os.Rename(configPath, backupPath); err != nil {
			return fmt.Errorf("backing up existing configuration: %w", err)
		}
		fmt.Printf("Backed up existing configuration to: %s\n", backupPath)
	}

	if err := os.WriteFile(configPath, []byte(jsonContent), 0644); err != nil {
		return fmt.Errorf("writing configuration to %s: %w", configPath, err)
	}
	return nil
}

func printInitSummary(configPath string, opts config.InitDevOptions) {
	detected := opts.Features.DetectedStacks()
	if len(detected) == 0 {
		detected = append(detected, "Generic Dev/Agent")
	}

	fmt.Printf("Initialized development sandbox configuration: %s\n", configPath)
	fmt.Printf("  Detected stack(s): %s\n", strings.Join(detected, ", "))
	if !opts.Features.EnableSSH {
		fmt.Printf("  SSH forwarding:    disabled\n")
	} else {
		fmt.Printf("  SSH forwarding:    enabled (host agent)\n")
	}
}
