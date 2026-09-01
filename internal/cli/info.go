package cli

import (
	"fmt"
	"os"
	"strings"

	"bws/internal/config"
	"bws/internal/profile"

	"github.com/fatih/color"
)

// ColorProfile formats a profile name with the distinctive profile color (bold cyan).
func ColorProfile(name string) string {
	return color.New(color.FgCyan, color.Bold).Sprint(name)
}

// HandleStatusShort displays the concise sandbox status focusing on installed profiles.
func HandleStatusShort() error {
	globalPath := config.GlobalPath()
	globalCfg, _ := config.LoadFile(globalPath)
	cwd, _ := os.Getwd()
	localPath := config.LocalPath()
	var localCfg *config.Config
	if fi, err := os.Stat(localPath); err == nil && !fi.IsDir() {
		if c, err := config.LoadFile(localPath); err == nil {
			localCfg = c
		}
	}
	mergedCfg := config.Merge(globalCfg, localCfg)

	if len(mergedCfg.Profiles) == 0 {
		fmt.Println("No capability profiles configured for the current workspace (using default base sandbox).")
		fmt.Printf("\nWorkspace: %s\n", cwd)
		fmt.Println("\nRun 'bws status all' to display full sandbox execution plan and mounts.")
		fmt.Println("Run 'bws status --help' for command options.")
		return nil
	}

	registry, err := profile.LoadRegistry(cwd)
	if err != nil {
		return err
	}
	ctx := profile.DetectMatchContext()

	seen := make(map[string]bool)
	var resolvedList []string
	for _, pName := range mergedCfg.Profiles {
		resolved, err := profile.ResolveProfile(pName, registry, ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error resolving profile %q: %v\n", pName, err)
			continue
		}
		for _, sub := range resolved.Profiles {
			if !seen[sub] {
				seen[sub] = true
				resolvedList = append(resolvedList, sub)
			}
		}
	}

	dim := color.New(color.FgHiBlack).SprintFunc()
	termWidth := getTerminalWidth()

	fmt.Println("Installed profiles (in order of application):")
	for i, pName := range resolvedList {
		p := registry[pName]
		if p == nil {
			fmt.Printf("  %2d. %s %s\n", i+1, ColorProfile(pName), dim("(unknown profile)"))
			continue
		}
		src := p.Source
		if src == "" {
			src = "embedded"
		}
		desc := p.Description
		if desc == "" {
			desc = "(no description)"
		}
		if len(p.Requires) > 0 {
			var coloredReqs []string
			for _, r := range p.Requires {
				coloredReqs = append(coloredReqs, ColorProfile(r))
			}
			desc += fmt.Sprintf(" [requires: %s]", strings.Join(coloredReqs, ", "))
		}

		prefix := fmt.Sprintf("  %2d. %-14s %-10s ", i+1, ColorProfile(pName), dim("["+src+"]"))
		plainPrefixLen := 6 + 14 + 1 + 10 + 1
		wrappedDesc := wrapText(desc, plainPrefixLen, termWidth)

		fmt.Printf("%s%s\n", prefix, wrappedDesc)
	}

	configInfo := "(global)"
	if _, err := os.Stat(localPath); err == nil {
		configInfo = "(.bws/config.jsonc)"
	}
	fmt.Printf("\nWorkspace: %s %s\n", cwd, dim(configInfo))
	fmt.Println("\nRun 'bws status all' to display full sandbox execution plan and mounts.")
	fmt.Println("Run 'bws status --help' for command options.")
	return nil
}
