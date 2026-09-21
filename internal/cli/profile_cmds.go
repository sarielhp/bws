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
func HandleProfileList(compoundOnly ...bool) error {
	cwd, _ := os.Getwd()
	registry, err := profile.LoadRegistry(cwd)
	if err != nil {
		return err
	}

	var uniqueProfiles []*profile.Profile
	seen := make(map[string]bool)
	for _, p := range registry {
		if len(compoundOnly) > 0 && compoundOnly[0] && p.Kind != "compound" {
			continue
		}
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
	if err := printCompoundDetails(p, registry); err != nil {
		return err
	}
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

	return printResolvedProfile(resolved)
}

func printResolvedProfile(resolved *profile.ResolvedProfile) error {
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

// PrintSynthesisSource displays the source intelligence found for profile synthesis.
func PrintSynthesisSource(name string, info profile.SynthesisInfo) {
	if info.HomebrewFormula && info.FirejailProfile {
		fmt.Printf("Synthesizing profile for %q from Homebrew and Firejail...\n", name)
	} else if info.HomebrewFormula {
		fmt.Printf("Synthesizing profile for %q from Homebrew formula...\n", name)
	} else if info.FirejailProfile {
		fmt.Printf("Synthesizing profile for %q from Firejail profile...\n", name)
	} else {
		fmt.Printf("No upstream recipes found in Homebrew or Firejail for %q.\n", name)
	}
}

// HandleProfileNew generates a new profile by querying Homebrew and Firejail intelligence.
// Verifies that the tool is installed in the system ($PATH) before writing the profile unless force is true.
func HandleProfileNew(name string, global, local, force bool) error {
	cwd, _ := os.Getwd()

	registry, err := profile.LoadRegistry(cwd)
	if err != nil {
		return err
	}

	p, info, err := profile.GenerateProfileDetailed(name, registry)
	if err != nil {
		return err
	}

	if !info.HomebrewFormula && !info.FirejailProfile && !force {
		return fmt.Errorf("cannot synthesize profile for %q: not found in Homebrew formulae or Firejail profiles (use -f/--force to generate an empty skeleton)", name)
	}

	PrintSynthesisSource(name, info)

	if binPath, err := profile.VerifyToolInstalled(name, p); err == nil {
		fmt.Printf("  • Host binary:  %s\n", binPath)
	} else {
		fmt.Printf("  • Host binary:  not found in $PATH (profile created without local installation)\n")
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
