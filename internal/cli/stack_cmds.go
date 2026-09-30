package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bws/internal/config"
	"bws/internal/policy"
	"bws/internal/profile"
	"bws/internal/stack"
	"bws/internal/util"
)

// HandleStackList displays all registered seed and user-saved stacks.
func HandleStackList(category string, jsonOutput bool) error {
	stacks, err := stack.List(category)
	if err != nil {
		return err
	}
	if jsonOutput {
		return writeJSON(os.Stdout, stacks)
	}
	if len(stacks) == 0 {
		if category != "" {
			fmt.Printf("No stacks found in category %q.\n", category)
		} else {
			fmt.Println("No stacks registered.")
		}
		return nil
	}
	var curated, userSaved []*stack.Stack
	for _, s := range stacks {
		if s.Source == "embedded" {
			curated = append(curated, s)
		} else {
			userSaved = append(userSaved, s)
		}
	}
	if len(curated) > 0 {
		fmt.Println("Curated Seed Stacks:")
		printStackTable(curated)
	}
	if len(userSaved) > 0 {
		if len(curated) > 0 {
			fmt.Println()
		}
		fmt.Println("User Saved Stacks:")
		printStackTable(userSaved)
	}
	return nil
}

func printStackTable(stacks []*stack.Stack) {
	for _, s := range stacks {
		title := s.Title
		if title == "" {
			title = s.Name
		}
		cat := s.Category
		if cat != "" {
			cat = " [" + cat + "]"
		}
		fmt.Printf("  %-16s %s%s\n", s.Name, title, cat)
		if s.Description != "" {
			fmt.Printf("                   %s\n", s.Description)
		}
		if len(s.Profiles) > 0 {
			fmt.Printf("                   Profiles: %s\n", strings.Join(s.Profiles, ", "))
		}
	}
}

// HandleStackShow outputs detailed metadata, profiles, and provenance for a stack.
func HandleStackShow(name string) error {
	stk, err := stack.Get(name)
	if err != nil {
		return err
	}
	digest, err := stack.Digest(stk)
	if err != nil {
		return err
	}
	fmt.Printf("Stack:        %s\n", stk.Name)
	if stk.Title != "" {
		fmt.Printf("Title:        %s\n", stk.Title)
	}
	if stk.Category != "" {
		fmt.Printf("Category:     %s\n", stk.Category)
	}
	fmt.Printf("Source:       %s\n", stk.Source)
	if stk.Description != "" {
		fmt.Printf("Description:  %s\n", stk.Description)
	}
	fmt.Printf("Profiles:     %s\n", strings.Join(stk.Profiles, ", "))
	if len(stk.DefaultCmd) > 0 {
		fmt.Printf("Default Cmd:  %s\n", strings.Join(stk.DefaultCmd, " "))
	}
	if len(stk.Env) > 0 {
		fmt.Printf("Environment:\n")
		for k, v := range stk.Env {
			fmt.Printf("  %s=%s\n", k, v)
		}
	}
	printStackFeatures(stk.Features)
	fmt.Printf("Digest:       %s\n", digest)
	if stk.Provenance != nil {
		fmt.Printf("Provenance:\n")
		if stk.Provenance.SourceWorkspace != "" {
			fmt.Printf("  Workspace:  %s\n", stk.Provenance.SourceWorkspace)
		}
		if !stk.Provenance.SavedAt.IsZero() {
			fmt.Printf("  Saved At:   %s\n", stk.Provenance.SavedAt.Format("2006-01-02 15:04:05 UTC"))
		}
		if stk.Provenance.VerifiedDigest != "" {
			fmt.Printf("  Verified:   %s\n", stk.Provenance.VerifiedDigest)
		}
	}
	return nil
}

func printStackFeatures(f *config.FeaturesConfig) {
	if f == nil {
		return
	}
	var active []string
	cfg := &config.Config{Features: f}
	if config.FeatureEnabledDefault(cfg, func(fc *config.FeaturesConfig) *bool { return fc.EnableSSH }, true) {
		active = append(active, "ssh")
	}
	if config.FeatureEnabledDefault(cfg, func(fc *config.FeaturesConfig) *bool { return fc.EnableX11 }, true) {
		active = append(active, "x11")
	}
	if config.FeatureEnabledDefault(cfg, func(fc *config.FeaturesConfig) *bool { return fc.EnableDBus }, false) {
		active = append(active, "dbus")
	}
	if config.FeatureEnabledDefault(cfg, func(fc *config.FeaturesConfig) *bool { return fc.MaskHistory }, false) {
		active = append(active, "mask-history")
	}
	if config.FeatureEnabledDefault(cfg, func(fc *config.FeaturesConfig) *bool { return fc.BlockGH }, true) {
		active = append(active, "block-gh")
	}
	if len(active) > 0 {
		fmt.Printf("Features:     %s\n", strings.Join(active, ", "))
	}
}

// HandleStackSave snapshots the active workspace into a reusable user stack.
func HandleStackSave(name, title, desc string, force, noVerify bool) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	root, localPath := config.FindWorkspaceRoot(cwd)
	if fi, err := os.Stat(localPath); err != nil || fi.IsDir() {
		return fmt.Errorf("genesis invariant violation: 'bws stack save' requires an active, existing bws workspace (no %s found)", localPath)
	}
	if err := stack.ValidateName(name); err != nil {
		return err
	}
	if existing, _ := stack.Get(name); existing != nil && !force {
		return fmt.Errorf("stack %q already exists; use --force to overwrite", name)
	}

	resolution, err := policy.Load(cwd)
	if err != nil {
		return err
	}
	registry, err := profile.LoadRegistry(cwd)
	if err != nil {
		return err
	}

	profiles, err := resolveConstituentProfiles(resolution, registry)
	if err != nil {
		return err
	}

	if !noVerify {
		if err := verifyConstituentProfiles(resolution.Config, cwd, profiles, registry); err != nil {
			return err
		}
	}

	sanitizedFeatures, sanitizedEnv := sanitizeStackConfig(resolution.Config, root)
	if title == "" {
		title = defaultStackTitle(name)
	}
	if desc == "" {
		desc = fmt.Sprintf("Saved persona from workspace %s", filepath.Base(root))
	}

	stk := &stack.Stack{
		Name:        name,
		Title:       title,
		Description: desc,
		Category:    "User Saved",
		Profiles:    profiles,
		DefaultCmd:  []string{"bash"},
		Features:    sanitizedFeatures,
		Env:         sanitizedEnv,
		Provenance: &stack.Provenance{
			SourceWorkspace: root,
			SavedAt:         time.Now().UTC(),
		},
	}
	digest, err := stack.Digest(stk)
	if err != nil {
		return err
	}
	stk.Provenance.VerifiedDigest = digest

	targetPath, err := stack.SaveUserStack(stk)
	if err != nil {
		return err
	}
	fmt.Printf("Saved stack %q to %s\n  Profiles:        %s\n  Verified Digest: %s\n",
		name, targetPath, strings.Join(profiles, ", "), digest[:12])
	PrintWorkspaceInfo(root)
	return nil
}

func resolveConstituentProfiles(r *policy.Resolution, reg map[string]*profile.Profile) ([]string, error) {
	var names []string
	if r.Local != nil && r.Local.Stack != "" {
		baseStk, err := stack.Get(r.Local.Stack)
		if err == nil {
			names = append(names, baseStk.Profiles...)
		}
	}
	if r.Local != nil && len(r.Local.Profiles) > 0 {
		names = append(names, r.Local.Profiles...)
	} else if len(names) == 0 && len(r.Config.Profiles) > 0 {
		names = append(names, r.Config.Profiles...)
	}
	names = uniqueNames(names)
	if len(names) == 0 {
		return nil, fmt.Errorf("cannot save stack: workspace has no configured profiles")
	}
	return names, nil
}

func verifyConstituentProfiles(cfg *config.Config, cwd string, profiles []string, reg map[string]*profile.Profile) error {
	for _, pName := range profiles {
		resolved, err := profile.ResolveProfile(pName, reg, profile.DetectMatchContext())
		if err != nil {
			return fmt.Errorf("resolving profile %q for verification: %w", pName, err)
		}
		if len(resolved.Tests) == 0 {
			continue
		}
		fmt.Fprintf(os.Stderr, "Running smoke tests for profile %q...\n", pName)
		results, err := profile.RunProfileTests(cfg, cwd, resolved, false)
		if err != nil {
			return fmt.Errorf("smoke test failed for profile %q: %w (bypass with --no-verify)", pName, err)
		}
		for _, res := range results {
			if res.Status == "failed" {
				return fmt.Errorf("smoke test %q for profile %q failed: %v (bypass with --no-verify)", res.Name, pName, res.Error)
			}
		}
	}
	return nil
}

func sanitizeStackConfig(cfg *config.Config, root string) (*config.FeaturesConfig, map[string]string) {
	sanitizedEnv := make(map[string]string)
	if cfg.Env != nil {
		for k, v := range cfg.Env {
			if policy.SensitiveEnv(k) || policy.RuntimeEnv(k) {
				continue
			}
			v = strings.ReplaceAll(v, root, ".")
			v = strings.ReplaceAll(v, util.HomeDir(), config.HomeToken)
			sanitizedEnv[k] = v
		}
	}
	var sanitizedFeatures *config.FeaturesConfig
	if cfg.Features != nil {
		f := *cfg.Features
		f.AutoInit = ""
		f.SSHKeys = nil
		sanitizedFeatures = &f
	}
	return sanitizedFeatures, sanitizedEnv
}

func defaultStackTitle(name string) string {
	parts := strings.Split(name, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

// HandleStackUpdate pulls upstream stack changes into the current workspace.
func HandleStackUpdate(dryRun, yes bool) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	root, localPath := config.FindWorkspaceRoot(cwd)
	before, err := config.PolicyBytes(localPath)
	if err != nil || before == nil {
		return fmt.Errorf("no workspace configuration found in %s", root)
	}
	localCfg, err := config.Parse(before, localPath)
	if err != nil {
		return fmt.Errorf("loading local config: %w", err)
	}
	if strings.TrimSpace(localCfg.Stack) == "" {
		return fmt.Errorf("current workspace does not configure a stack (stack is unset)")
	}

	upstream, err := stack.Get(localCfg.Stack)
	if err != nil {
		return fmt.Errorf("upstream stack %q not found: %w", localCfg.Stack, err)
	}
	upstreamDigest, err := stack.Digest(upstream)
	if err != nil {
		return fmt.Errorf("computing upstream digest: %w", err)
	}

	currentDigest := ""
	if localCfg.ReviewedStack != nil {
		currentDigest = localCfg.ReviewedStack.SHA256
	}
	if currentDigest == upstreamDigest {
		fmt.Printf("Workspace stack %q is already up to date (%s).\n", upstream.Name, upstreamDigest[:12])
		return nil
	}

	printStackUpdateDetails(upstream, currentDigest, upstreamDigest)

	if dryRun {
		fmt.Println("[dry-run] Stack update available. Run 'bws stack update' to apply.")
		return nil
	}

	if !yes && IsInteractiveTTY(int(os.Stdin.Fd())) {
		if err := confirmPolicy(os.Stdin, os.Stderr, "Apply upstream stack update to this workspace?"); err != nil {
			return err
		}
	}

	backupPath := localPath + ".bak"
	priorBak, _ := config.PolicyBytes(backupPath)
	if err := config.AtomicPolicyWrite(backupPath, before, priorBak); err != nil {
		return fmt.Errorf("creating backup %s: %w", backupPath, err)
	}
	fmt.Printf("Created configuration backup: %s\n", backupPath)

	localCfg.ReviewedStack = &config.ProfileApproval{
		Source: upstream.Source,
		SHA256: upstreamDigest,
	}

	updatedJSON, err := json.MarshalIndent(localCfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling updated config: %w", err)
	}
	updatedJSON = append(updatedJSON, '\n')

	if err := config.AtomicPolicyWrite(localPath, updatedJSON, before); err != nil {
		return fmt.Errorf("writing updated config: %w", err)
	}
	fmt.Printf("Successfully updated stack %q in %s (%s)\n", upstream.Name, localPath, upstreamDigest[:12])
	PrintWorkspaceInfo(root)
	return nil
}

func printStackUpdateDetails(upstream *stack.Stack, currentDigest, upstreamDigest string) {
	fmt.Fprintf(os.Stderr, "Stack: %s (%s)\n", upstream.Name, upstream.Title)
	if currentDigest != "" {
		fmt.Fprintf(os.Stderr, "Current approved digest:  %s\n", currentDigest)
	} else {
		fmt.Fprintf(os.Stderr, "Current approved digest:  (unapproved)\n")
	}
	fmt.Fprintf(os.Stderr, "Upstream approved digest: %s\n", upstreamDigest)
	fmt.Fprintf(os.Stderr, "Base profiles:            %s\n", strings.Join(upstream.Profiles, ", "))
}
