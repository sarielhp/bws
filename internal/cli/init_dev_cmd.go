package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bw/internal/config"
	"bw/internal/profile"
)

// HandleInitDev handles the `bw init-dev` command.
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

	opts := config.InitDevOptions{
		Features:     features,
		TargetDir:    absDir,
		Force:        force,
		DryRun:       dryRun,
		NoSSH:        noSSH,
		OpenCode:     opencode,
		Preset:       preset,
		Profiles:     finalActiveProfiles,
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
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", filepath.Dir(configPath), err)
	}

	if _, err := os.Stat(configPath); err == nil {
		if !force {
			backupPath := configPath + ".bak"
			if err := os.Rename(configPath, backupPath); err != nil {
				return fmt.Errorf("backing up existing configuration: %w", err)
			}
			fmt.Printf("Backed up existing configuration to: %s\n", backupPath)
		}
	}

	if err := os.WriteFile(configPath, []byte(jsonContent), 0644); err != nil {
		return fmt.Errorf("writing configuration to %s: %w", configPath, err)
	}

	var detected []string
	if opts.Features.HasGo {
		detected = append(detected, "Go")
	}
	if opts.Features.HasPython {
		detected = append(detected, "Python/UV")
	}
	if opts.Features.HasRust {
		detected = append(detected, "Rust")
	}
	if opts.Features.HasNode {
		detected = append(detected, "Node")
	}
	if opts.Features.HasLatex {
		detected = append(detected, "LaTeX/TeX")
	}
	if opts.Features.HasOpenCode {
		detected = append(detected, "OpenCode")
	}
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

	return nil
}
