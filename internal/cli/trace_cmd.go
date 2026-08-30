package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bws/internal/config"
	"bws/internal/profile"
	"bws/internal/trace"

	"github.com/fatih/color"
)

// HandleTrace runs a target command under strace, analyzes captured syscalls,
// and optionally writes discovered mounts/features to config or a profile.
func HandleTrace(targetCmd []string, dryRun, writeConfig bool, profileName string, global, local, force, verbose bool) error {
	if len(targetCmd) == 0 {
		return fmt.Errorf("no target command specified (e.g. bws trace -- myapp --help)")
	}

	cwd, _ := os.Getwd()
	homeDir, _ := os.UserHomeDir()

	fmt.Printf("Tracing command: %s\n", strings.Join(targetCmd, " "))
	fmt.Println(strings.Repeat("=", 60))

	res, err := trace.RunTrace(trace.TraceOptions{
		Command: targetCmd,
		WorkDir: cwd,
		HomeDir: homeDir,
		Verbose: verbose,
	})
	if err != nil {
		return fmt.Errorf("running trace: %w", err)
	}

	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Trace analysis complete (process exited with code %d):\n\n", res.ExitCode)

	printTraceSummary(res)

	// Case 1: Standalone profile generation (-p / --profile)
	if profileName != "" {
		return handleProfileGeneration(res, profileName, global, local, force, dryRun)
	}

	// Case 2: Direct configuration writing (-w / --write)
	if writeConfig {
		return handleConfigWrite(res, global, local, dryRun)
	}

	// Case 3: Dry run / Preview mode (default if neither -w nor -p)
	printTracePreview(res)
	return nil
}

func printTraceSummary(res *trace.TraceResult) {
	cyan := color.New(color.FgCyan, color.Bold).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	dim := color.New(color.FgHiBlack).SprintFunc()

	fmt.Println(cyan("Detected Features:"))
	fmt.Printf("  • Network (TCP/UDP):   %s\n", boolStatus(res.Features.Net, green, dim))
	fmt.Printf("  • SSH Agent:           %s\n", boolStatus(res.Features.SSH, green, dim))
	fmt.Printf("  • D-Bus Session/Bus:   %s\n", boolStatus(res.Features.DBus, green, dim))
	fmt.Printf("  • X11 Display Server:  %s\n", boolStatus(res.Features.X11, green, dim))
	fmt.Printf("  • WSL2 Interop:        %s\n", boolStatus(res.Features.WSL, green, dim))
	fmt.Println()

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

func handleProfileGeneration(res *trace.TraceResult, profileName string, global, local, force, dryRun bool) error {
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
	if local {
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

func handleConfigWrite(res *trace.TraceResult, global, local, dryRun bool) error {
	targetPath := configFilePath(global)

	if dryRun {
		fmt.Printf("Configuration changes that would be applied to %s:\n", targetPath)
		printTraceConfigSnippet(res)
		return nil
	}

	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		if err := config.CreateDefault(targetPath); err != nil {
			return fmt.Errorf("creating default config: %w", err)
		}
	}

	addedRW := 0
	for _, b := range res.BindsRW {
		entry := fmt.Sprintf("%q", b)
		if err := config.AddBindArrayElement(targetPath, "binds_rw", entry); err == nil {
			addedRW++
		}
	}

	addedRO := 0
	for _, b := range res.BindsRO {
		entry := fmt.Sprintf("%q", b)
		if err := config.AddBindArrayElement(targetPath, "binds_ro", entry); err == nil {
			addedRO++
		}
	}

	var enabledFeatures []string
	if res.Features.SSH {
		_ = config.SetConfigKV(targetPath, "enable_ssh", "true")
		enabledFeatures = append(enabledFeatures, "enable_ssh")
	}
	if res.Features.DBus {
		_ = config.SetConfigKV(targetPath, "enable_dbus", "true")
		enabledFeatures = append(enabledFeatures, "enable_dbus")
	}
	if res.Features.X11 {
		_ = config.SetConfigKV(targetPath, "enable_x11", "true")
		enabledFeatures = append(enabledFeatures, "enable_x11")
	}
	if res.Features.WSL {
		_ = config.SetConfigKV(targetPath, "enable_wsl", "true")
		enabledFeatures = append(enabledFeatures, "enable_wsl")
	}

	label := "local"
	if global {
		label = "global"
	}

	fmt.Printf("✓ Updated %s configuration (%s):\n", label, targetPath)
	fmt.Printf("  • Added %d read-write bind mounts\n", addedRW)
	fmt.Printf("  • Added %d read-only bind mounts\n", addedRO)
	if len(enabledFeatures) > 0 {
		fmt.Printf("  • Enabled features: %s\n", strings.Join(enabledFeatures, ", "))
	}
	return nil
}

func printTracePreview(res *trace.TraceResult) {
	fmt.Println("Suggested configuration snippet:")
	printTraceConfigSnippet(res)
	fmt.Println("\nTip:")
	fmt.Println("  • Use -w to write directly to your .bws/config.jsonc")
	fmt.Println("  • Use -p <name> to save as a reusable capability profile in profiles/<name>.json")
}

func printTraceConfigSnippet(res *trace.TraceResult) {
	cfgSnippet := make(map[string]interface{})
	if len(res.BindsRW) > 0 {
		cfgSnippet["binds_rw"] = res.BindsRW
	}
	if len(res.BindsRO) > 0 {
		cfgSnippet["binds_ro"] = res.BindsRO
	}
	features := make(map[string]bool)
	if res.Features.SSH {
		features["enable_ssh"] = true
	}
	if res.Features.DBus {
		features["enable_dbus"] = true
	}
	if res.Features.X11 {
		features["enable_x11"] = true
	}
	if res.Features.WSL {
		features["enable_wsl"] = true
	}
	if len(features) > 0 {
		cfgSnippet["features"] = features
	}

	data, _ := json.MarshalIndent(cfgSnippet, "", "  ")
	fmt.Println(string(data))
}
