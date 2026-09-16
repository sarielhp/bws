package policy

import (
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"

	"bws/internal/config"
	"bws/internal/profile"
	"bws/internal/util"
)

// ExportOptions describes a declarative snapshot, never a process snapshot.
type ExportOptions struct {
	Name        string
	Description string
	Flatten     bool
}

// ExportPlan contains the proposed profile and explicit portability limitations.
type ExportPlan struct {
	Profile      *profile.Profile
	Unsupported  []string
	MachinePaths []string
}

// Export preserves supported effective settings and verifies their reapplication.
func Export(r *Resolution, registry map[string]*profile.Profile, opts ExportOptions) (*ExportPlan, error) {
	if err := profile.ValidateName(opts.Name); err != nil {
		return nil, err
	}
	cfg := config.Clone(r.Config)
	if err := profile.ValidateMounts(cfg); err != nil {
		return nil, err
	}
	p := snapshot(cfg, opts.Name, opts.Description, r.Workspace)
	if !opts.Flatten {
		p.Requires = slices.Clone(cfg.Profiles)
		if old := registry[opts.Name]; old != nil {
			p.Aliases = slices.Clone(old.Aliases)
			var requires []string
			for _, name := range p.Requires {
				if registry[name] == old {
					requires = append(requires, old.Requires...)
				} else {
					requires = append(requires, name)
				}
			}
			p.Requires = requires
		}
		if err := subtractDependencies(p, registry, r.Workspace); err != nil {
			return nil, err
		}
	}
	plan := &ExportPlan{Profile: p, Unsupported: unsupportedSettings(cfg)}
	if err := checkExportEnvironment(cfg); err != nil {
		return nil, err
	}
	plan.MachinePaths = machinePaths(cfg, r.Workspace)
	if err := profile.Pin(p, registry); err != nil {
		return nil, err
	}
	if err := verifyExport(r, p, registry); err != nil {
		return nil, fmt.Errorf("cannot represent this setup as selected profile references: %w; try --flatten or resolve the conflicting global declaration", err)
	}
	return plan, nil
}

func snapshot(c *config.Config, name, description, workspace string) *profile.Profile {
	if description == "" {
		description = "Reusable sandbox capabilities"
	}
	normalize := func(s string) string { return portablePath(s, workspace) }
	p := &profile.Profile{Name: name, Kind: "compound", Description: description, Features: config.Clone(c).Features, Env: map[string]string{}}
	p.Path, p.Mask, p.Copy = mapStrings(c.Path, normalize), mapStrings(c.Mask, normalize), mapStrings(c.Copy, normalize)
	p.PassEnv = slices.Clone(c.PassEnv)
	for k, v := range c.Env {
		if !runtimeEnv(k) {
			p.Env[k] = normalize(v)
		}
	}
	for _, b := range c.BindsRW {
		p.BindsRW = append(p.BindsRW, portableBind(b, normalize))
	}
	for _, b := range c.BindsRO {
		p.BindsRO = append(p.BindsRO, portableBind(b, normalize))
	}
	if p.Features != nil {
		p.Features.SSHKeys = mapStrings(p.Features.SSHKeys, normalize)
		p.Features.AutoInit = ""
	}
	return p
}

func portableBind(b config.BindEntry, normalize func(string) string) []string {
	dest := b.Sandbox
	if dest == "" {
		dest = b.Host
	}
	return []string{normalize(b.Host), normalize(dest)}
}

func portablePath(s, workspace string) string {
	path := util.ExpandHome(strings.ReplaceAll(s, config.HomeToken, util.HomeDir()))
	for _, candidate := range []struct{ root, token string }{{workspace, WorkspaceToken}, {util.HomeDir(), config.HomeToken}} {
		if path == candidate.root {
			return candidate.token
		}
		if strings.HasPrefix(path, candidate.root+string(filepath.Separator)) {
			return candidate.token + strings.TrimPrefix(path, candidate.root)
		}
	}
	return s
}

func mapStrings(values []string, fn func(string) string) []string {
	var out []string
	for _, value := range values {
		out = append(out, fn(value))
	}
	return out
}

func subtractDependencies(p *profile.Profile, registry map[string]*profile.Profile, root string) error {
	deps := &config.Config{Profiles: p.Requires}
	if err := ApplyRegistry(deps, registry); err != nil {
		return err
	}
	base := snapshot(deps, "", "", root)
	p.Path = withoutStrings(p.Path, base.Path)
	p.Mask = withoutStrings(p.Mask, base.Mask)
	p.Copy = withoutStrings(p.Copy, base.Copy)
	p.PassEnv = withoutStrings(p.PassEnv, base.PassEnv)
	p.BindsRW = withoutBinds(p.BindsRW, base.BindsRW)
	p.BindsRO = withoutBinds(p.BindsRO, base.BindsRO)
	for k, v := range base.Env {
		if p.Env[k] == v {
			delete(p.Env, k)
		}
	}
	return nil
}

func withoutStrings(values, base []string) []string {
	return slices.DeleteFunc(values, func(s string) bool { return slices.Contains(base, s) })
}

func withoutBinds(values, base [][]string) [][]string {
	return slices.DeleteFunc(values, func(b []string) bool {
		return slices.ContainsFunc(base, func(v []string) bool { return slices.Equal(v, b) })
	})
}

func verifyExport(r *Resolution, p *profile.Profile, registry map[string]*profile.Profile) error {
	reg := make(map[string]*profile.Profile, len(registry)+1)
	for k, v := range registry {
		reg[k] = v
	}
	reg[p.Name] = p
	replay := config.Merge(r.Global, &config.Config{Profiles: []string{p.Name}})
	if err := ApplyRegistry(replay, reg); err != nil {
		return err
	}
	ExpandWorkspace(replay, r.Workspace)
	want, got := comparable(r.Config, r.Workspace), comparable(replay, r.Workspace)
	for _, field := range []string{"path", "mask", "copy", "pass_env", "env", "binds_rw", "binds_ro", "features"} {
		if !reflect.DeepEqual(want[field], got[field]) {
			return fmt.Errorf("%s does not round-trip", field)
		}
	}
	return nil
}

func comparable(c *config.Config, root string) map[string]any {
	p := snapshot(config.Clone(c), "", "", root)
	dedup := func(v []string) []string {
		out := []string{}
		for _, s := range v {
			if !slices.Contains(out, s) {
				out = append(out, s)
			}
		}
		return out
	}
	binds := func(v [][]string) []string {
		var out []string
		for _, b := range v {
			out = append(out, strings.Join(b, "\x00"))
		}
		out = dedup(out)
		sort.Strings(out)
		return out
	}
	mask, copyPaths, pass := dedup(p.Mask), dedup(p.Copy), dedup(p.PassEnv)
	sort.Strings(mask)
	sort.Strings(copyPaths)
	sort.Strings(pass)
	return map[string]any{"path": dedup(p.Path), "mask": mask, "copy": copyPaths, "pass_env": pass,
		"env": p.Env, "binds_rw": binds(p.BindsRW), "binds_ro": binds(p.BindsRO), "features": comparableFeatures(p.Features)}
}

func comparableFeatures(f *config.FeaturesConfig) *config.FeaturesConfig {
	if f == nil {
		return &config.FeaturesConfig{}
	}
	copy := *f
	copy.AutoInit = ""
	return &copy
}
