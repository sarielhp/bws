package policy

import (
	"fmt"
	"strings"

	"bws/internal/config"
	"bws/internal/stack"
)

// ApplyStack resolves cfg.Stack if configured: verifies approval fingerprint,
// merges base profiles before local profiles, and applies stack features/env.
func ApplyStack(cfg *config.Config, currentDir string) error {
	if cfg == nil || strings.TrimSpace(cfg.Stack) == "" {
		return nil
	}
	stk, err := stack.Get(cfg.Stack)
	if err != nil {
		return fmt.Errorf("loading stack %q: %w", cfg.Stack, err)
	}
	if cfg.ReviewedStack != nil {
		digest, err := stack.Digest(stk)
		if err != nil {
			return err
		}
		if cfg.ReviewedStack.SHA256 != digest {
			return fmt.Errorf("stack %q changed or was shadowed; review with 'bws stack show %s', then update with 'bws stack update'", cfg.Stack, cfg.Stack)
		}
	}
	cfg.Profiles = MergeStackProfiles(stk.Profiles, cfg.Profiles)
	if stk.Features != nil {
		cfg.Features = config.MergeFeatures(stk.Features, cfg.Features)
	}
	if stk.Env != nil {
		if cfg.Env == nil {
			cfg.Env = make(map[string]string)
		}
		for k, v := range stk.Env {
			if _, exists := cfg.Env[k]; !exists {
				cfg.Env[k] = v
			}
		}
	}
	if cfg.Env == nil {
		cfg.Env = make(map[string]string)
	}
	cfg.Env["BWS_ACTIVE_STACK"] = stk.Name
	return nil
}

// MergeStackProfiles combines stack base profiles with local additive profiles, preserving order and uniqueness.
func MergeStackProfiles(stackProfiles, localProfiles []string) []string {
	seen := make(map[string]bool)
	var merged []string
	for _, p := range stackProfiles {
		if !seen[p] {
			seen[p] = true
			merged = append(merged, p)
		}
	}
	for _, p := range localProfiles {
		if !seen[p] {
			seen[p] = true
			merged = append(merged, p)
		}
	}
	return merged
}
