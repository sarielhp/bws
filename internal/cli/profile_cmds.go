package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"bw/internal/config"
	"bw/internal/profile"
	"bw/internal/util"
)

// HandleProfileList lists all registered profiles and their source.
func HandleProfileList() error {
	cwd, _ := os.Getwd()
	registry, err := profile.LoadRegistry(cwd)
	if err != nil {
		return err
	}

	var names []string
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)

	fmt.Println("Available Sandbox Profiles:")
	for _, name := range names {
		p := registry[name]
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
			reqStr = fmt.Sprintf(" [requires: %s]", strings.Join(p.Requires, ", "))
		}
		fmt.Printf("  • %-12s [%-8s] %s%s\n", name, src, desc, reqStr)
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

	fmt.Printf("Profile: %s\n", p.Name)
	fmt.Printf("Source:  %s\n", p.Source)
	if p.Description != "" {
		fmt.Printf("Description: %s\n", p.Description)
	}
	if len(p.Requires) > 0 {
		fmt.Printf("Requires:    %s\n", strings.Join(p.Requires, ", "))
	}
	if len(resolved.Profiles) > 1 {
		fmt.Printf("Resolved Chain: %s\n", strings.Join(resolved.Profiles, " -> "))
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

	fmt.Println("\nRun 'bw profile test " + name + "' to verify in sandbox.")
	return nil
}

// HandleProfileTest runs all verification and smoke tests for a profile.
func HandleProfileTest(name string, verbose bool) error {
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

	globalPath := config.GlobalPath()
	baseCfg, err := config.LoadFile(globalPath)
	if err != nil {
		baseCfg = &config.Config{
			System: &config.SystemConfig{
				ShareNet: boolPtr(true),
			},
		}
	}
	localPath := config.LocalPath()
	if fi, err := os.Stat(localPath); err == nil && !fi.IsDir() {
		if localCfg, err := config.LoadFile(localPath); err == nil {
			baseCfg = config.Merge(baseCfg, localCfg)
		}
	}

	fmt.Printf("Testing profile %q in sandbox:\n", name)
	results, err := profile.RunProfileTests(baseCfg, cwd, resolved, verbose)
	if err != nil {
		return err
	}

	passedCount := 0
	skippedCount := 0
	failedCount := 0

	for _, r := range results {
		switch r.Status {
		case "passed":
			passedCount++
			fmt.Printf("  ✓ %-35s (%s)\n", r.Name, r.Duration.Round(time.Millisecond))
		case "skipped":
			skippedCount++
			fmt.Printf("  ⊘ %-35s [skipped: %s]\n", r.Name, r.Output)
		case "failed":
			failedCount++
			fmt.Printf("  ✗ %-35s [FAILED]\n", r.Name)
			if r.Output != "" {
				fmt.Printf("    Output:\n      %s\n", strings.ReplaceAll(r.Output, "\n", "\n      "))
			}
		}
	}

	fmt.Println()
	if failedCount > 0 {
		return fmt.Errorf("profile %q test failed: %d passed, %d skipped, %d failed", name, passedCount, skippedCount, failedCount)
	}

	fmt.Printf("Summary: %d passed, %d skipped. Everything is fine.\n", passedCount, skippedCount)
	return nil
}

// HandleProfileSearch searches local profiles, host tools, and Homebrew formula registry.
func HandleProfileSearch(query string) error {
	cleanQ := strings.ToLower(strings.TrimSpace(query))
	if cleanQ == "" {
		return fmt.Errorf("search query cannot be empty")
	}

	cwd, _ := os.Getwd()
	registry, _ := profile.LoadRegistry(cwd)

	fmt.Printf("Searching for %q:\n\n", query)

	// 1. Local / Embedded Profiles
	var matchedProfiles []string
	for name, p := range registry {
		if strings.Contains(strings.ToLower(name), cleanQ) || strings.Contains(strings.ToLower(p.Description), cleanQ) {
			matchedProfiles = append(matchedProfiles, name)
		}
	}
	sort.Strings(matchedProfiles)

	if len(matchedProfiles) > 0 {
		fmt.Println("Local & Embedded Profiles:")
		for _, name := range matchedProfiles {
			p := registry[name]
			src := p.Source
			if src == "" {
				src = "embedded"
			}
			fmt.Printf("  ★ %-12s [%-8s] %s\n", name, src, p.Description)
		}
		fmt.Println()
	}

	// 2. Host Binary check
	if util.CommandExists(cleanQ) {
		fmt.Printf("Host Executable: %q is installed on this system.\n\n", cleanQ)
	}

	// 3. Query Homebrew Index
	client := &http.Client{Timeout: 3 * time.Second}
	hbURL := fmt.Sprintf("https://formulae.brew.sh/api/formula/%s.json", cleanQ)
	if resp, err := client.Get(hbURL); err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var hb struct {
			Name string `json:"name"`
			Desc string `json:"desc"`
		}
		if err := json.Unmarshal(body, &hb); err == nil && hb.Name != "" {
			fmt.Println("Available on Homebrew:")
			fmt.Printf("  • %-12s [Formula]  %s\n", hb.Name, hb.Desc)
			fmt.Printf("    Run 'bw profile new %s' to generate a sandbox profile.\n\n", hb.Name)
		}
	}

	return nil
}

func boolPtr(b bool) *bool {
	return &b
}
