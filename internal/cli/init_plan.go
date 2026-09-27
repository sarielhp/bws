package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"

	"bws/internal/config"
	"bws/internal/detect"
	"bws/internal/policy"
	"bws/internal/profile"
	"bws/internal/stack"
)

// InitPlan is a side-effect-free configuration and its effective permission preview.
type InitPlan struct {
	Data      []byte
	Effective *config.Config
}

// BuildInitPlan stores references and approval fingerprints, not expanded mounts.
func BuildInitPlan(root string, names []string, flags policy.Flags) (*InitPlan, error) {
	return BuildInitPlanWithStack(root, "", names, flags)
}

// BuildInitPlanWithStack stores references, stack configuration, and approval fingerprints.
func BuildInitPlanWithStack(root string, stackName string, names []string, flags policy.Flags) (*InitPlan, error) {
	sources, err := policy.LoadSources(root)
	if err != nil {
		return nil, err
	}
	registry, err := profile.LoadRegistry(root)
	if err != nil {
		return nil, err
	}
	pins, err := profile.PinSelections(names, registry)
	if err != nil {
		return nil, err
	}

	var approval *config.ProfileApproval
	if stackName != "" {
		stk, err := stack.Get(stackName)
		if err != nil {
			return nil, fmt.Errorf("resolving stack %q: %w", stackName, err)
		}
		digest, err := stack.Digest(stk)
		if err != nil {
			return nil, err
		}
		approval = &config.ProfileApproval{
			Source: stk.Source,
			SHA256: digest,
		}
	}

	cfg := &config.Config{
		Stack:            stackName,
		ReviewedStack:    approval,
		Profiles:         names,
		ReviewedProfiles: pins,
	}
	flags.Apply(cfg)
	effective, err := policy.Resolve(sources.Global, cfg, root)
	if err != nil {
		return nil, err
	}
	flags.Apply(effective)

	data, err := json.MarshalIndent(struct {
		Stack         string                            `json:"stack,omitempty"`
		ReviewedStack *config.ProfileApproval           `json:"reviewed_stack,omitempty"`
		Profiles      []string                          `json:"profiles,omitempty"`
		Reviewed      map[string]config.ProfileApproval `json:"reviewed_profiles,omitempty"`
		Features      *config.FeaturesConfig            `json:"features,omitempty"`
	}{
		Stack:         stackName,
		ReviewedStack: approval,
		Profiles:      names,
		Reviewed:      pins,
		Features:      cfg.Features,
	}, "", "  ")
	if err != nil {
		return nil, err
	}
	return &InitPlan{append(data, '\n'), effective}, nil
}

func selectInitStackAndProfiles(root string, opts InitOptions) (string, []string, error) {
	if opts.Stack != "" {
		stk, err := stack.Get(opts.Stack)
		if err != nil {
			return "", nil, fmt.Errorf("stack %q not found: %w", opts.Stack, err)
		}
		names := slices.Clone(opts.Profiles)
		if opts.OpenCode {
			names = append(names, "opencode")
		}
		return stk.Name, uniqueNames(names), nil
	}

	if len(opts.Profiles) > 0 || opts.Preset != "" || opts.Basic || opts.OpenCode {
		names, err := selectInitProfiles(root, opts)
		return "", names, err
	}

	evidence, err := detect.Scan(root)
	if err != nil {
		return "", nil, err
	}
	if isCleanWorkspace(evidence) {
		return promptCleanWorkspaceStacks(opts)
	}
	return promptRankedWorkspaceStacks(root, evidence, opts)
}

func isCleanWorkspace(e *detect.Evidence) bool {
	if len(e.Entries) == 0 {
		return true
	}
	for _, entry := range e.Entries {
		parts := strings.Split(entry.Path, "/")
		isDot := false
		for _, part := range parts {
			if strings.HasPrefix(part, ".") {
				isDot = true
				break
			}
		}
		if !isDot {
			return false
		}
	}
	return true
}

func promptCleanWorkspaceStacks(opts InitOptions) (string, []string, error) {
	stacks, err := stack.List("")
	if err != nil {
		return "", nil, err
	}
	if len(stacks) == 0 {
		return "", nil, fmt.Errorf("no stacks available")
	}
	var curated, userSaved []*stack.Stack
	for _, s := range stacks {
		if s.Source == "embedded" {
			curated = append(curated, s)
		} else {
			userSaved = append(userSaved, s)
		}
	}

	if IsInteractiveTTY(int(os.Stdin.Fd())) && !opts.DryRun {
		stackName, names, err := interactiveCleanStacks(curated, userSaved)
		if err == nil {
			return stackName, names, nil
		}
		if !strings.Contains(err.Error(), "terminal dimensions too small") && !strings.Contains(err.Error(), "non-interactive") {
			return "", nil, err
		}
	}
	return fallbackCleanWorkspaceStacks(curated, userSaved, opts)
}

func interactiveCleanStacks(curated, userSaved []*stack.Stack) (string, []string, error) {
	var choices []StackChoice
	for _, s := range curated {
		choices = append(choices, StackChoice{Stack: s, Badge: "Curated"})
	}
	for _, s := range userSaved {
		choices = append(choices, StackChoice{Stack: s, Badge: "Saved"})
	}
	choices = append(choices, StackChoice{
		Label:   "Basic (raw profiles)",
		IsBasic: true,
		Badge:   "Profiles",
	})
	idx, err := SelectStackInteractive("Select an environment stack for this clean workspace", choices)
	if err != nil {
		return "", nil, err
	}
	if choices[idx].IsBasic {
		return "", nil, nil
	}
	return choices[idx].Stack.Name, nil, nil
}

func fallbackCleanWorkspaceStacks(curated, userSaved []*stack.Stack, opts InitOptions) (string, []string, error) {
	ordered := append(append([]*stack.Stack{}, curated...), userSaved...)
	fmt.Fprintln(os.Stderr, "Workspace is clean/empty. Available environment stacks:")
	if len(curated) > 0 {
		fmt.Fprintln(os.Stderr, "Curated Seed Stacks:")
		for i, s := range curated {
			fmt.Fprintf(os.Stderr, "  %2d. %-15s [%s] — %s\n", i+1, s.Name, s.Category, s.Description)
		}
	}
	if len(userSaved) > 0 {
		fmt.Fprintln(os.Stderr, "User Saved Stacks:")
		for i, s := range userSaved {
			idx := len(curated) + i + 1
			fmt.Fprintf(os.Stderr, "  %2d. %-15s [%s] — %s\n", idx, s.Name, s.Category, s.Description)
		}
	}

	if opts.DryRun || !IsInteractiveTTY(int(os.Stdin.Fd())) {
		return "", nil, fmt.Errorf("select a stack with --stack <name>, or profiles with --profile <name>; use 'bws stack list' to inspect candidates")
	}

	fmt.Fprint(os.Stderr, "Choose a stack number, name, or Enter to cancel: ")
	line, err := readPolicyAnswer(os.Stdin)
	if err != nil {
		return "", nil, fmt.Errorf("cancelled: %w", err)
	}
	choice := strings.TrimSpace(line)
	if choice == "" {
		return "", nil, fmt.Errorf("cancelled; no changes written")
	}
	if n, err := strconv.Atoi(choice); err == nil {
		if n < 1 || n > len(ordered) {
			return "", nil, fmt.Errorf("invalid stack selection %d", n)
		}
		return ordered[n-1].Name, nil, nil
	}
	for _, s := range ordered {
		if strings.EqualFold(s.Name, choice) {
			return s.Name, nil, nil
		}
	}
	return "", nil, fmt.Errorf("unknown stack %q", choice)
}

type rankedStack struct {
	Stack    *stack.Stack
	Rank     int
	Evidence []string
}

func rankWorkspaceStacks(evidence *detect.Evidence, stacks []*stack.Stack, reg map[string]*profile.Profile) []rankedStack {
	suggestions, _ := profile.Suggestions(evidence, reg, false)
	profRank := make(map[string]int)
	profEvidence := make(map[string][]string)
	for _, s := range suggestions {
		profRank[s.Name] = s.Rank
		profEvidence[s.Name] = s.Evidence
	}

	var ranked []rankedStack
	for _, stk := range stacks {
		totalRank := 0
		var reasons []string
		for _, pName := range stk.Profiles {
			if r, ok := profRank[pName]; ok {
				totalRank += r
				reasons = append(reasons, profEvidence[pName]...)
			}
		}
		ranked = append(ranked, rankedStack{
			Stack:    stk,
			Rank:     totalRank,
			Evidence: uniqueNames(reasons),
		})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].Rank != ranked[j].Rank {
			return ranked[i].Rank > ranked[j].Rank
		}
		return ranked[i].Stack.Name < ranked[j].Stack.Name
	})
	return ranked
}

func promptRankedWorkspaceStacks(root string, evidence *detect.Evidence, opts InitOptions) (string, []string, error) {
	stacks, err := stack.List("")
	if err != nil {
		return "", nil, err
	}
	registry, err := profile.LoadRegistry(root)
	if err != nil {
		return "", nil, err
	}
	ranked := rankWorkspaceStacks(evidence, stacks, registry)
	var recommended []rankedStack
	for _, r := range ranked {
		if r.Rank > 0 {
			recommended = append(recommended, r)
		}
	}
	if len(recommended) == 0 {
		names, err := selectInitProfiles(root, opts)
		return "", names, err
	}

	if IsInteractiveTTY(int(os.Stdin.Fd())) && !opts.DryRun {
		stackName, names, err := interactiveRankedStacks(root, recommended, stacks)
		if err == nil {
			return stackName, names, nil
		}
		if !strings.Contains(err.Error(), "terminal dimensions too small") && !strings.Contains(err.Error(), "non-interactive") {
			return "", nil, err
		}
	}
	return fallbackRankedWorkspaceStacks(root, recommended, stacks, opts)
}

func interactiveRankedStacks(root string, recommended []rankedStack, all []*stack.Stack) (string, []string, error) {
	var choices []StackChoice
	seen := make(map[string]bool)
	for _, r := range recommended {
		seen[r.Stack.Name] = true
		badge := "Matches " + strings.Join(r.Evidence, ", ")
		choices = append(choices, StackChoice{Stack: r.Stack, Badge: badge})
	}
	for _, s := range all {
		if !seen[s.Name] {
			choices = append(choices, StackChoice{Stack: s, Badge: s.Category})
		}
	}
	choices = append(choices, StackChoice{
		Label:   "Basic (detected tool profiles)",
		IsBasic: true,
		Badge:   "Profiles",
	})
	idx, err := SelectStackInteractive("Recommended Stacks for this workspace", choices)
	if err != nil {
		return "", nil, err
	}
	if choices[idx].IsBasic {
		names, err := basicProfiles(root)
		return "", names, err
	}
	return choices[idx].Stack.Name, nil, nil
}

func fallbackRankedWorkspaceStacks(root string, recommended []rankedStack, stacks []*stack.Stack, opts InitOptions) (string, []string, error) {
	fmt.Fprintln(os.Stderr, "Recommended Stacks for this workspace:")
	for i, r := range recommended {
		evidenceStr := strings.Join(r.Evidence, ", ")
		title := r.Stack.Title
		if title == "" {
			title = r.Stack.Name
		}
		fmt.Fprintf(os.Stderr, "  %2d. %-15s (matches %s) — %s\n", i+1, r.Stack.Name, evidenceStr, title)
		if len(r.Stack.Profiles) > 0 {
			fmt.Fprintf(os.Stderr, "      Profiles: %s\n", strings.Join(r.Stack.Profiles, ", "))
		}
	}

	if opts.DryRun || !IsInteractiveTTY(int(os.Stdin.Fd())) {
		return "", nil, fmt.Errorf("select a stack explicitly with --stack <name>, or request detected tool profiles with --basic; use 'bws stack list' to inspect candidates")
	}

	fmt.Fprint(os.Stderr, "Choose a stack number, name, 'basic' for profiles, or Enter to cancel: ")
	line, err := readPolicyAnswer(os.Stdin)
	if err != nil {
		return "", nil, fmt.Errorf("cancelled: %w", err)
	}
	choice := strings.TrimSpace(line)
	if choice == "" {
		return "", nil, fmt.Errorf("cancelled; no changes written")
	}
	if choice == "basic" {
		names, err := basicProfiles(root)
		return "", names, err
	}
	if n, err := strconv.Atoi(choice); err == nil {
		if n < 1 || n > len(recommended) {
			return "", nil, fmt.Errorf("invalid stack selection %d", n)
		}
		return recommended[n-1].Stack.Name, nil, nil
	}
	for _, r := range recommended {
		if strings.EqualFold(r.Stack.Name, choice) {
			return r.Stack.Name, nil, nil
		}
	}
	for _, s := range stacks {
		if strings.EqualFold(s.Name, choice) {
			return s.Name, nil, nil
		}
	}
	return "", nil, fmt.Errorf("unknown stack %q", choice)
}

func selectInitProfiles(root string, opts InitOptions) ([]string, error) {
	names := slices.Clone(opts.Profiles)
	if opts.Preset != "" {
		preset, err := presetProfiles(opts.Preset)
		if err != nil {
			return nil, err
		}
		names = append(names, preset...)
	}
	if opts.OpenCode {
		names = append(names, "opencode")
	}
	if len(names) > 0 {
		return uniqueNames(names), nil
	}
	if opts.Basic {
		return basicProfiles(root)
	}
	report, err := SuggestProfiles(root, true)
	if err != nil {
		return nil, err
	}
	for i, s := range report.Suggestions {
		fmt.Fprintf(os.Stderr, "  %d. %s — %s; includes %s\n", i+1, s.Name, strings.Join(s.Evidence, ", "), strings.Join(s.Requires, ", "))
	}
	if opts.DryRun || !IsInteractiveTTY(int(os.Stdin.Fd())) {
		return nil, fmt.Errorf("select profiles explicitly with --profile <name>, or request detected tool profiles with --basic; use 'bws profile suggest' to inspect candidates")
	}
	fmt.Fprint(os.Stderr, "Choose a number, profile names, 'basic', or Enter to cancel: ")
	line, err := readPolicyAnswer(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("cancelled: %w", err)
	}
	choice := strings.TrimSpace(line)
	if choice == "" {
		return nil, fmt.Errorf("cancelled; no changes written")
	}
	if choice == "basic" {
		return basicProfiles(root)
	}
	if n, err := strconv.Atoi(choice); err == nil {
		if n < 1 || n > len(report.Suggestions) {
			return nil, fmt.Errorf("invalid selection")
		}
		return []string{report.Suggestions[n-1].Name}, nil
	}
	return uniqueNames(strings.Split(choice, ",")), nil
}

func basicProfiles(root string) ([]string, error) {
	evidence, err := detect.Scan(root)
	if err != nil {
		return nil, err
	}
	registry, err := profile.LoadRegistry(root)
	if err != nil {
		return nil, err
	}
	matches, err := profile.Suggestions(evidence, registry, false)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, m := range matches {
		// Only embedded tool profiles participate in unattended legacy auto-init.
		if m.Kind != "compound" && m.Source == "embedded" && m.Rank > 1 {
			names = append(names, m.Name)
		}
	}
	return uniqueNames(names), nil
}

func uniqueNames(names []string) []string {
	out := []string{}
	for _, name := range names {
		clean := strings.TrimSpace(name)
		if clean != "" && !slices.Contains(out, clean) {
			out = append(out, clean)
		}
	}
	return out
}

func presetProfiles(name string) ([]string, error) {
	presets := map[string][]string{
		"go": {"go"}, "python": {"python", "uv"}, "rust": {"rust"}, "node": {"node"},
		"latex": {"latex"}, "agent": {"opencode"},
		"all": {"go", "python", "uv", "rust", "node", "latex", "opencode"},
	}
	names, ok := presets[name]
	if !ok {
		return nil, fmt.Errorf("unknown preset %q", name)
	}
	return names, nil
}
