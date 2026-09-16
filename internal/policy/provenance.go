package policy

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"bws/internal/config"
	"bws/internal/profile"
)

// Origins maps permission declarations to their sources, in application order.
// Values and credentials are not included in environment/feature keys.
type Origins map[string][]string

// Explain records the sources participating in a resolved policy.
func Explain(r *Resolution, registry map[string]*profile.Profile) (Origins, error) {
	out := Origins{}
	out.addConfig(r.Global, "global: "+r.GlobalPath, r.Workspace)
	names := config.Merge(r.Global, r.Local).Profiles
	seen := map[string]bool{}
	for _, name := range names {
		resolved, err := profile.ResolveProfile(name, registry, profile.DetectMatchContext())
		if err != nil {
			return nil, err
		}
		for _, dep := range resolved.Profiles {
			if seen[dep] {
				continue
			}
			seen[dep] = true
			p := registry[dep]
			source := fmt.Sprintf("%s profile: %s", p.Source, p.Name)
			c := &config.Config{Env: p.Env, Path: p.Path, PassEnv: p.PassEnv, Mask: p.Mask, Copy: p.Copy, Features: p.Features}
			for _, b := range p.BindsRW {
				c.BindsRW = append(c.BindsRW, config.BindEntry{Host: b[0], Sandbox: b[1]})
			}
			for _, b := range p.BindsRO {
				c.BindsRO = append(c.BindsRO, config.BindEntry{Host: b[0], Sandbox: b[1]})
			}
			out.addConfig(c, source, r.Workspace)
			for _, rule := range p.Rules {
				if !profile.MatchRule(rule, profile.DetectMatchContext()) {
					continue
				}
				conditional := &config.Config{Env: rule.Env, Path: rule.Path}
				for _, b := range rule.BindsRW {
					conditional.BindsRW = append(conditional.BindsRW, config.BindEntry{Host: b[0], Sandbox: b[1]})
				}
				for _, b := range rule.BindsRO {
					conditional.BindsRO = append(conditional.BindsRO, config.BindEntry{Host: b[0], Sandbox: b[1]})
				}
				out.addConfig(conditional, source+" (host rule)", r.Workspace)
			}
		}
	}
	out.addConfig(r.Local, "local: "+r.LocalPath, r.Workspace)
	return out, nil
}

func (o Origins) add(key, source string) {
	if !slices.Contains(o[key], source) {
		o[key] = append(o[key], source)
	}
}

func (o Origins) addConfig(c *config.Config, source, root string) {
	if c == nil {
		return
	}
	for key := range c.Env {
		o.add("env."+key, source)
	}
	for _, b := range c.BindsRW {
		o.add(bindOriginKey(b, root), source)
	}
	for _, b := range c.BindsRO {
		o.add("read-only "+strings.TrimPrefix(bindOriginKey(b, root), "writable "), source)
	}
	for _, group := range []struct {
		key    string
		values []string
	}{{"path ", c.Path}, {"mask ", c.Mask}, {"copy ", c.Copy}, {"pass_env ", c.PassEnv}} {
		for _, v := range group.values {
			o.add(group.key+portablePath(v, root), source)
		}
	}
	if c.Features != nil {
		data, err := json.Marshal(c.Features)
		if err != nil {
			return
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(data, &fields); err != nil {
			return
		}
		for k, v := range fields {
			if string(v) != "null" && string(v) != `""` {
				o.add("features."+k, source)
			}
		}
	}
}

func bindOriginKey(b config.BindEntry, root string) string {
	normalized := portableBind(b, func(s string) string { return portablePath(s, root) })
	return "writable " + strings.Join(normalized, " -> ")
}

// WritableOrigins identifies declarations responsible for an external writable bind.
func WritableOrigins(origins Origins, b config.BindEntry, root string) []string {
	return origins[bindOriginKey(b, root)]
}
