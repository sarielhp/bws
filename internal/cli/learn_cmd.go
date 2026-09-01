package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bws/internal/config"
	"bws/internal/learn"
	"bws/internal/profile"

	"github.com/fatih/color"
)

// HandleLearn runs a target command under strace, analyzes captured syscalls,
// diffs against the active sandbox config, and applies or previews the delta.
func HandleLearn(targetCmd []string, dryRun bool, profileName string, global, force, verbose bool) error {
	if len(targetCmd) == 0 {
		return fmt.Errorf("no target command specified (e.g. bws learn -- myapp --help)")
	}

	cwd, _ := os.Getwd()
	homeDir, _ := os.UserHomeDir()

	fmt.Printf("Learning command: %s\n", strings.Join(targetCmd, " "))
	fmt.Println(strings.Repeat("=", 60))

	res, err := learn.RunTrace(learn.TraceOptions{
		Command: targetCmd,
		WorkDir: cwd,
		HomeDir: homeDir,
		Verbose: verbose,
	})
	if err != nil {
		return fmt.Errorf("running learn trace: %w", err)
	}

	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Trace analysis complete (process exited with code %d):\n\n", res.ExitCode)

	// Load target configuration for diffing
	targetPath := configFilePath(global)
	var targetConfig *config.Config
	if _, err := os.Stat(targetPath); err == nil {
		if c, err := config.LoadFile(targetPath); err == nil {
			targetConfig = c
		}
	}

	delta := learn.ComputeDelta(res, targetConfig, homeDir)

	printLearnSummary(res, delta)

	// Case 1: Standalone profile generation (-p / --profile)
	if profileName != "" {
		return handleProfileGeneration(res, profileName, global, force, dryRun)
	}

	// Case 2: Dry run / Preview mode (-n / --dry-run)
	if dryRun {
		if delta.IsEmpty() {
			fmt.Println("✓ Sandbox configuration already covers all required access. No changes needed.")
			return nil
		}
		fmt.Printf("Discovered sandbox configuration additions for %s:\n\n", targetPath)
		printDeltaSnippet(delta)
		return nil
	}

	// Case 3: Default live merge
	if delta.IsEmpty() {
		fmt.Println("✓ Sandbox configuration already covers all required access. No changes needed.")
		return nil
	}

	return handleLiveMerge(targetPath, delta, global)
}

func printLearnSummary(res *learn.TraceResult, delta *learn.Delta) {
	cyan := color.New(color.FgCyan, color.Bold).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	red := color.New(color.FgRed, color.Bold).SprintFunc()
	dim := color.New(color.FgHiBlack).SprintFunc()

	if len(delta.SecurityAlerts) > 0 {
		for _, alert := range delta.SecurityAlerts {
			fmt.Println(red(alert))
		}
		fmt.Println()
	}

	fmt.Println(cyan("Detected Features:"))
	fmt.Printf("  • Network (TCP/UDP):   %s\n", boolStatus(res.Features.Net, green, dim))
	fmt.Printf("  • SSH Agent:           %s\n", boolStatus(res.Features.SSH, green, dim))
	fmt.Printf("  • D-Bus Session/Bus:   %s\n", boolStatus(res.Features.DBus, green, dim))
	fmt.Printf("  • X11 Display Server:  %s\n", boolStatus(res.Features.X11, green, dim))
	fmt.Printf("  • WSL2 Interop:        %s\n", boolStatus(res.Features.WSL, green, dim))
	fmt.Println()

	if res.DiscoveredPath != "" {
		fmt.Println(cyan("Binary PATH Discovery:"))
		fmt.Printf("  • Binary directory:    %s\n\n", yellow(res.DiscoveredPath))
	}

	fmt.Println(cyan("Discovered Bind Mounts:"))
	if len(res.BindsRW) > 0 {
		fmt.Printf("  %s:\n", yellow("Read-Write (binds_rw)"))
		for _, b := range res.BindsRW {
			fmt.Printf("    • %s\n", b)
		}
	} else {
		fmt.Printf("  %s: %s\n", yellow("Read-Write (binds_rw)"), dim("(none outside workspace)"))
	}

	if len(res.BindsRO) > 0 {
		fmt.Printf("  %s:\n", green("Read-Only (binds_ro)"))
		for _, b := range res.BindsRO {
			fmt.Printf("    • %s\n", b)
		}
	} else {
		fmt.Printf("  %s: %s\n", green("Read-Only (binds_ro)"), dim("(none)"))
	}
	fmt.Println()
}

func boolStatus(b bool, active, inactive func(...interface{}) string) string {
	if b {
		return active("yes (detected)")
	}
	return inactive("no")
}

func handleProfileGeneration(res *learn.TraceResult, profileName string, global, force, dryRun bool) error {
	cleanName := strings.ToLower(strings.TrimSpace(profileName))
	p := res.ToProfile(cleanName)

	if dryRun {
		data, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println("Generated capability profile preview:")
		fmt.Println(string(data))
		return nil
	}

	var targetDir string
	if !global {
		cwd, _ := os.Getwd()
		targetDir = profile.LocalProfilesDir(cwd)
	} else {
		targetDir = profile.GlobalProfilesDir()
	}

	targetPath := filepath.Join(targetDir, cleanName+".json")
	if _, err := os.Stat(targetPath); err == nil && !force {
		return fmt.Errorf("profile %q already exists at %s; use -f/--force to overwrite", cleanName, targetPath)
	}

	if err := profile.SaveProfile(p, targetPath); err != nil {
		return fmt.Errorf("saving profile: %w", err)
	}

	fmt.Printf("✓ Saved profile %s to: %s\n", ColorProfile(cleanName), targetPath)
	fmt.Println("\nNext steps:")
	fmt.Printf("  • Verify profile in sandbox: bws profile test %s\n", cleanName)
	fmt.Printf("  • Enable in workspace:       bws add %s\n", cleanName)
	return nil
}

func handleLiveMerge(targetPath string, delta *learn.Delta, global bool) error {
	mergeRes, err := learn.ApplyDelta(targetPath, delta)
	if err != nil {
		return fmt.Errorf("merging learned delta into %s: %w", targetPath, err)
	}

	label := "local"
	if global {
		label = "global"
	}

	fmt.Printf("✓ Updated %s configuration (%s):\n", label, targetPath)
	if mergeRes.AddedRW > 0 {
		fmt.Printf("  • Added %d read-write bind mounts\n", mergeRes.AddedRW)
	}
	if mergeRes.AddedRO > 0 {
		fmt.Printf("  • Added %d read-only bind mounts\n", mergeRes.AddedRO)
	}
	if mergeRes.UpgradedRO > 0 {
		fmt.Printf("  • Upgraded %d mounts (read-only -> read-write)\n", mergeRes.UpgradedRO)
	}
	if mergeRes.AddedPath > 0 {
		fmt.Printf("  • Added %d binary PATH entries\n", mergeRes.AddedPath)
	}
	if len(mergeRes.EnabledFeatures) > 0 {
		fmt.Printf("  • Enabled features: %s\n", strings.Join(mergeRes.EnabledFeatures, ", "))
	}
	return nil
}

func printDeltaSnippet(delta *learn.Delta) {
	cfgSnippet := make(map[string]interface{})
	if len(delta.Path) > 0 {
		cfgSnippet["path"] = delta.Path
	}
	if len(delta.BindsRW) > 0 {
		cfgSnippet["binds_rw"] = delta.BindsRW
	}
	if len(delta.BindsRO) > 0 {
		cfgSnippet["binds_ro"] = delta.BindsRO
	}
	features := make(map[string]bool)
	if delta.Features.SSH {
		features["enable_ssh"] = true
	}
	if delta.Features.DBus {
		features["enable_dbus"] = true
	}
	if delta.Features.X11 {
		features["enable_x11"] = true
	}
	if delta.Features.WSL {
		features["enable_wsl"] = true
	}
	if len(features) > 0 {
		cfgSnippet["features"] = features
	}

	data, _ := json.MarshalIndent(cfgSnippet, "", "  ")
	fmt.Println(string(data))
}
