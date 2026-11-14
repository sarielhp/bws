package policy

import (
	"bws/internal/config"
	"bws/internal/profile"
	"fmt"
	"slices"
	"strings"
)

// mergeResolvedProfile adds a resolved profile's settings to cfg. List entries
// already present are skipped and existing env values win.
func mergeResolvedProfile(cfg *config.Config, resolved *profile.ResolvedProfile) {
	cfg.PassEnv = appendNew(cfg.PassEnv, resolved.PassEnv...)
	cfg.Mask = appendNew(cfg.Mask, resolved.Mask...)
	cfg.Copy = appendNew(cfg.Copy, resolved.Copy...)
	cfg.Path = appendNew(cfg.Path, resolved.Path...)
	cfg.BindsRW = appendNewBinds(cfg.BindsRW, resolved.BindsRW)
	cfg.BindsRO = appendNewBinds(cfg.BindsRO, resolved.BindsRO)
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

// appendNew appends the items not already in list, keeping order.
func appendNew[T comparable](list []T, items ...T) []T {
	for _, item := range items {
		if !slices.Contains(list, item) {
			list = append(list, item)
		}
	}
	return list
}

// appendNewBinds appends profile binds whose host and sandbox pair is not
// already bound.
func appendNewBinds(list []config.BindEntry, binds [][]string) []config.BindEntry {
	for _, b := range binds {
		exists := slices.ContainsFunc(list, func(e config.BindEntry) bool { return e.Host == b[0] && e.Sandbox == b[1] })
		if !exists {
			list = append(list, config.BindEntry{Host: b[0], Sandbox: b[1]})
		}
	}
	return list
}

// ApplyProfiles applies cfg's stack and selected profiles to cfg, resolving
// them from the registry visible from currentDir.
func ApplyProfiles(cfg *config.Config, currentDir string) error {
	if cfg.Stack != "" {
		if err := ApplyStack(cfg); err != nil {
			return err
		}
	}
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

	var allResolved []string

	for _, pName := range cfg.Profiles {
		resolved, err := profile.ResolveProfile(pName, registry, ctx)
		if err != nil {
			return fmt.Errorf("resolving profile %q: %w", pName, err)
		}
		allResolved = appendNew(allResolved, resolved.Profiles...)
		mergeResolvedProfile(cfg, resolved)
		if registry[pName].Kind == "compound" {
			if cfg.Env == nil {
				cfg.Env = map[string]string{}
			}
			for key, value := range resolved.Env {
				cfg.Env[key] = value
			}
		}
	}
	cfg.ActiveProfiles = allResolved
	if len(allResolved) > 0 {
		if cfg.Env == nil {
			cfg.Env = make(map[string]string)
		}
		cfg.Env["BWS_ACTIVE_PROFILES"] = strings.Join(allResolved, ",")
	}
	return nil
}
