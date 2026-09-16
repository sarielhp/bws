package policy

import (
	"bws/internal/config"
	"bws/internal/profile"
	"fmt"
	"strings"
)

func mergeResolvedProfile(cfg *config.Config, resolved *profile.ResolvedProfile, seenPassEnv, seenMask, seenCopy, seenRW, seenRO, seenPath map[string]bool) {
	for _, pe := range resolved.PassEnv {
		if !seenPassEnv[pe] {
			seenPassEnv[pe] = true
			cfg.PassEnv = append(cfg.PassEnv, pe)
		}
	}
	for _, m := range resolved.Mask {
		if !seenMask[m] {
			seenMask[m] = true
			cfg.Mask = append(cfg.Mask, m)
		}
	}
	for _, c := range resolved.Copy {
		if !seenCopy[c] {
			seenCopy[c] = true
			cfg.Copy = append(cfg.Copy, c)
		}
	}
	for _, b := range resolved.BindsRW {
		key := b[0] + "->" + b[1]
		if !seenRW[key] {
			seenRW[key] = true
			cfg.BindsRW = append(cfg.BindsRW, config.BindEntry{Host: b[0], Sandbox: b[1]})
		}
	}
	for _, b := range resolved.BindsRO {
		key := b[0] + "->" + b[1]
		if !seenRO[key] {
			seenRO[key] = true
			cfg.BindsRO = append(cfg.BindsRO, config.BindEntry{Host: b[0], Sandbox: b[1]})
		}
	}
	for _, pt := range resolved.Path {
		if !seenPath[pt] {
			seenPath[pt] = true
			cfg.Path = append(cfg.Path, pt)
		}
	}
	for k, v := range resolved.Env {
		if cfg.Env == nil {
			cfg.Env = make(map[string]string)
		}
		if _, exists := cfg.Env[k]; !exists {
			cfg.Env[k] = v
		}
	}
	if resolved.Features != nil {
		cfg.Features = config.MergeFeatures(cfg.Features, resolved.Features)
	}
	if resolved.UnshareNet {
		if cfg.Features == nil {
			cfg.Features = &config.FeaturesConfig{}
		}
		t := true
		cfg.Features.NoNet = &t
	}
}

func ApplyProfiles(cfg *config.Config, currentDir string, verbose bool) error {
	if len(cfg.Profiles) == 0 {
		return nil
	}
	registry, err := profile.LoadRegistry(currentDir)
	if err != nil {
		return err
	}
	return ApplyRegistry(cfg, registry)
}

// ApplyRegistry applies an already-loaded registry using the launch merge rules.
func ApplyRegistry(cfg *config.Config, registry map[string]*profile.Profile) error {
	active, err := profile.PinSelections(cfg.Profiles, registry)
	if err != nil {
		return err
	}
	for name, pin := range cfg.ReviewedProfiles {
		if current, ok := active[name]; ok && pin != current {
			return fmt.Errorf("selected profile %q changed or was shadowed; review with 'bws profile show %s', then reinitialize with 'bws init --force --profile <selection>'", name, name)
		}
	}
	ctx := profile.DetectMatchContext()
	selection := &profile.Profile{Name: "selected profiles", Kind: "compound", Requires: cfg.Profiles, Env: cfg.Env}
	if err := profile.ValidateComposition(selection, registry, ctx); err != nil {
		return err
	}

	seenRW := make(map[string]bool)
	for _, b := range cfg.BindsRW {
		seenRW[b.Host+"->"+b.Sandbox] = true
	}
	seenRO := make(map[string]bool)
	for _, b := range cfg.BindsRO {
		seenRO[b.Host+"->"+b.Sandbox] = true
	}
	seenPath := make(map[string]bool)
	for _, p := range cfg.Path {
		seenPath[p] = true
	}
	seenPassEnv := make(map[string]bool)
	for _, pe := range cfg.PassEnv {
		seenPassEnv[pe] = true
	}
	seenMask := make(map[string]bool)
	for _, m := range cfg.Mask {
		seenMask[m] = true
	}
	seenCopy := make(map[string]bool)
	for _, c := range cfg.Copy {
		seenCopy[c] = true
	}

	var allResolved []string
	seenResolved := make(map[string]bool)

	for _, pName := range cfg.Profiles {
		resolved, err := profile.ResolveProfile(pName, registry, ctx)
		if err != nil {
			return fmt.Errorf("resolving profile %q: %w", pName, err)
		}
		for _, rName := range resolved.Profiles {
			if !seenResolved[rName] {
				seenResolved[rName] = true
				allResolved = append(allResolved, rName)
			}
		}
		mergeResolvedProfile(cfg, resolved, seenPassEnv, seenMask, seenCopy, seenRW, seenRO, seenPath)
		if registry[pName].Kind == "compound" {
			if cfg.Env == nil {
				cfg.Env = map[string]string{}
			}
			for key, value := range resolved.Env {
				cfg.Env[key] = value
			}
		}
	}
	if len(allResolved) > 0 {
		if cfg.Env == nil {
			cfg.Env = make(map[string]string)
		}
		cfg.Env["BWS_ACTIVE_PROFILES"] = strings.Join(allResolved, ",")
	}
	return nil
}
