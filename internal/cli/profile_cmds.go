package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"bws/internal/profile"

	"github.com/fatih/color"
)

// HandleProfileList lists all registered profiles and their source.
func HandleProfileList() error {
	cwd, _ := os.Getwd()
	registry, err := profile.LoadRegistry(cwd)
	if err != nil {
		return err
	}

	var uniqueProfiles []*profile.Profile
	seen := make(map[string]bool)
	for _, p := range registry {
		if !seen[p.Name] {
			seen[p.Name] = true
			uniqueProfiles = append(uniqueProfiles, p)
		}
	}
	sort.Slice(uniqueProfiles, func(i, j int) bool {
		return uniqueProfiles[i].Name < uniqueProfiles[j].Name
	})

	dim := color.New(color.FgHiBlack).SprintFunc()
	fmt.Println("Available sandbox profiles:")
	for _, p := range uniqueProfiles {
		src := p.Source
		if src == "" {
			src = "embedded"
		}
		desc := p.Description
		if desc == "" {
			desc = "(no description)"
		}
		reqStr := ""
		if len(p.Requires) > 0 {
			var coloredReqs []string
			for _, r := range p.Requires {
				coloredReqs = append(coloredReqs, ColorProfile(r))
			}
			reqStr = fmt.Sprintf(" [requires: %s]", strings.Join(coloredReqs, ", "))
		}
		aliasStr := ""
		if len(p.Aliases) > 0 {
			var coloredAliases []string
			for _, a := range p.Aliases {
				coloredAliases = append(coloredAliases, ColorProfile(a))
			}
			aliasStr = fmt.Sprintf(" (aliases: %s)", strings.Join(coloredAliases, ", "))
		}
		fmt.Printf("  • %-12s [%-8s] %s%s%s\n", ColorProfile(p.Name), dim(src), desc, reqStr, aliasStr)
	}
	return nil
}

// HandleProfileShow displays full configuration for a named profile.
func HandleProfileShow(name string) error {
	cwd, _ := os.Getwd()
	registry, err := profile.LoadRegistry(cwd)
	if err != nil {
		return err
	}

	ctx := profile.DetectMatchContext()
	resolved, err := profile.ResolveProfile(name, registry, ctx)
	if err != nil {
		return err
	}

	p := registry[name]

	fmt.Printf("Profile: %s\n", ColorProfile(p.Name))
	fmt.Printf("Source:  %s\n", p.Source)
	if p.Description != "" {
		fmt.Printf("Description: %s\n", p.Description)
	}
	if len(p.Requires) > 0 {
		var coloredReqs []string
		for _, r := range p.Requires {
			coloredReqs = append(coloredReqs, ColorProfile(r))
		}
		fmt.Printf("Requires:    %s\n", strings.Join(coloredReqs, ", "))
	}
	if len(resolved.Profiles) > 1 {
		var coloredChain []string
		for _, cp := range resolved.Profiles {
			coloredChain = append(coloredChain, ColorProfile(cp))
		}
		fmt.Printf("Resolved Chain: %s\n", strings.Join(coloredChain, " -> "))
	}

	if len(resolved.Path) > 0 {
		fmt.Println("\nPath Additions:")
		for _, pt := range resolved.Path {
			fmt.Printf("  + %s\n", pt)
		}
	}

	if len(resolved.BindsRW) > 0 {
		fmt.Println("\nRead-Write Binds:")
		for _, b := range resolved.BindsRW {
			fmt.Printf("  [RW] %s -> %s\n", b[0], b[1])
		}
	}

	if len(resolved.BindsRO) > 0 {
		fmt.Println("\nRead-Only Binds:")
		for _, b := range resolved.BindsRO {
			fmt.Printf("  [RO] %s -> %s\n", b[0], b[1])
		}
	}

	if len(resolved.PassEnv) > 0 {
		fmt.Println("\nPass-Through Environment:")
		for _, pe := range resolved.PassEnv {
			fmt.Printf("  $ %s\n", pe)
		}
	}

	if len(resolved.Mask) > 0 {
		fmt.Println("\nMasked / Hidden Paths:")
		for _, m := range resolved.Mask {
			fmt.Printf("  ⊘ %s\n", m)
		}
	}

	if len(resolved.Tests) > 0 {
		fmt.Println("\nVerification Tests:")
		for _, t := range resolved.Tests {
			opt := ""
			if t.Optional {
				opt = " (optional)"
			}
			fmt.Printf("  ✓ %s: %s%s\n", t.Name, strings.Join(t.Cmd, " "), opt)
		}
	}

	return nil
}

// HandleProfileNew generates a new profile by querying Homebrew and Firejail intelligence.
func HandleProfileNew(name string, global, local bool) error {
	cwd, _ := os.Getwd()
	fmt.Printf("Synthesizing profile for %q from Homebrew and Firejail...\n", name)

	p, err := profile.GenerateProfile(name)
	if err != nil {
		return err
	}

	var targetDir string
	if local {
		targetDir = profile.LocalProfilesDir(cwd)
	} else {
		targetDir = profile.GlobalProfilesDir()
	}

	targetPath := filepath.Join(targetDir, name+".json")
	if err := profile.SaveProfile(p, targetPath); err != nil {
		return err
	}

	fmt.Printf("✓ Saved profile to: %s\n", targetPath)
	fmt.Printf("  Description: %s\n", p.Description)
	if len(p.BindsRW) > 0 {
		fmt.Printf("  RW Binds:    %d paths\n", len(p.BindsRW))
	}
	if len(p.BindsRO) > 0 {
		fmt.Printf("  RO Binds:    %d paths\n", len(p.BindsRO))
	}
	if len(p.Tests) > 0 {
		fmt.Printf("  Tests:       %d checks\n", len(p.Tests))
	}

	fmt.Println("\nRun 'bws profile test " + name + "' to verify in sandbox.")
	return nil
}

// HandleProfileTest runs all verification and smoke tests for a profile.
