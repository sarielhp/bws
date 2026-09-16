package profile

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"bws/internal/config"
	"bws/internal/util"
)

// ValidateComposition requires compound authors to resolve ambiguous grants.
func ValidateComposition(p *Profile, registry map[string]*Profile, ctx MatchContext) error {
	if p.Kind != "compound" {
		return nil
	}
	env := map[string]string{}
	owners := map[string]string{}
	features := map[string]string{}
	explicit := featureValues(p.Features)
	binds := map[string]string{}
	for _, name := range p.Requires {
		r, err := ResolveProfile(name, registry, ctx)
		if err != nil {
			return err
		}
		for k, v := range r.Env {
			if old, ok := env[k]; ok && old != v {
				if _, overridden := p.Env[k]; !overridden {
					return fmt.Errorf("compound %s: conflicting environment %s from %s and %s; set an explicit env override", p.Name, k, owners[k], name)
				}
			}
			env[k], owners[k] = v, name
		}
		for k, v := range featureValues(r.Features) {
			if old, ok := features[k]; ok && old != v {
				if _, overridden := explicit[k]; !overridden {
					return fmt.Errorf("compound %s: conflicting feature %s; set an explicit feature override", p.Name, k)
				}
			}
			features[k] = v
		}
		if err := checkBinds(binds, r.BindsRW, "rw"); err != nil {
			return fmt.Errorf("compound %s: %w", p.Name, err)
		}
		if err := checkBinds(binds, r.BindsRO, "ro"); err != nil {
			return fmt.Errorf("compound %s: %w", p.Name, err)
		}
	}
	if err := checkBinds(binds, p.BindsRW, "rw"); err != nil {
		return err
	}
	return checkBinds(binds, p.BindsRO, "ro")
}

func featureValues(f *config.FeaturesConfig) map[string]string {
	result := map[string]string{}
	if f == nil {
		return result
	}
	data, err := json.Marshal(f)
	if err != nil {
		return result
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(data, &values); err != nil {
		return result
	}
	for k, v := range values {
		if string(v) != "null" && string(v) != `""` {
			result[k] = string(v)
		}
	}
	return result
}

func canonicalPath(path string) string {
	return filepath.Clean(util.ExpandHome(strings.ReplaceAll(path, config.HomeToken, util.HomeDir())))
}

func checkBinds(seen map[string]string, entries [][]string, mode string) error {
	for _, b := range entries {
		if len(b) != 2 {
			return fmt.Errorf("bind requires [host, destination]")
		}
		dest := canonicalPath(b[1])
		value := canonicalPath(b[0]) + " (" + mode + ")"
		if old, ok := seen[dest]; ok && old != value {
			return fmt.Errorf("conflicting mounts at %s: %s and %s; remove or change the conflicting declaration", b[1], old, value)
		}
		seen[dest] = value
	}
	return nil
}

// ValidateMounts refuses ambiguous equal-destination grants before exporting.
func ValidateMounts(cfg *config.Config) error {
	seen := map[string]string{}
	for _, group := range []struct {
		entries []config.BindEntry
		mode    string
	}{{cfg.BindsRO, "ro"}, {cfg.BindsRW, "rw"}} {
		for _, b := range group.entries {
			dest := b.Sandbox
			if dest == "" {
				dest = b.Host
			}
			if err := checkBinds(seen, [][]string{{b.Host, dest}}, group.mode); err != nil {
				return err
			}
		}
	}
	return nil
}
