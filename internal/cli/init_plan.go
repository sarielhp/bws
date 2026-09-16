package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"bws/internal/config"
	"bws/internal/detect"
	"bws/internal/policy"
	"bws/internal/profile"
)

// InitPlan is a side-effect-free configuration and its effective permission preview.
type InitPlan struct {
	Data      []byte
	Effective *config.Config
}

// BuildInitPlan stores references and approval fingerprints, not expanded mounts.
func BuildInitPlan(root string, names []string, flags policy.Flags) (*InitPlan, error) {
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
	cfg := &config.Config{Profiles: names, ReviewedProfiles: pins}
	flags.Apply(cfg)
	effective, err := policy.Resolve(sources.Global, cfg, root)
	if err != nil {
		return nil, err
	}
	flags.Apply(effective)
	// Launch flags become explicit local feature overrides, applied after profiles.
	data, err := json.MarshalIndent(struct {
		Profiles []string                          `json:"profiles"`
		Reviewed map[string]config.ProfileApproval `json:"reviewed_profiles,omitempty"`
		Features *config.FeaturesConfig            `json:"features,omitempty"`
	}{names, pins, cfg.Features}, "", "  ")
	if err != nil {
		return nil, err
	}
	return &InitPlan{append(data, '\n'), effective}, nil
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
