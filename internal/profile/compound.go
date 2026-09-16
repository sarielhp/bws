package profile

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"

	"bws/internal/config"
)

// Dependency records the source and content of a reviewed dependency.
type Dependency = config.ProfileApproval

// PinSelections captures selected roots as well as their transitive dependencies.
func PinSelections(names []string, registry map[string]*Profile) (map[string]Dependency, error) {
	result := map[string]Dependency{}
	for _, name := range names {
		order, err := dependencyOrder(name, registry)
		if err != nil {
			return nil, err
		}
		for _, dep := range order {
			value, err := Fingerprint(registry[dep])
			if err != nil {
				return nil, err
			}
			result[dep] = value
		}
	}
	return result, nil
}

var validName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

// ValidateName prevents path traversal and ambiguous profile filenames.
func ValidateName(name string) error {
	if !validName.MatchString(name) || len(name) > 100 {
		return fmt.Errorf("invalid profile name %q: use letters, digits, dots, underscores or hyphens", name)
	}
	return nil
}

// ValidateDefinition validates metadata before it can influence selection.
func ValidateDefinition(p *Profile) error {
	if err := ValidateName(p.Name); err != nil {
		return err
	}
	if p.Kind != "" && p.Kind != "compound" {
		return fmt.Errorf("profile %s: unknown kind %q", p.Name, p.Kind)
	}
	for _, name := range append(append([]string{}, p.Aliases...), p.Requires...) {
		if err := ValidateName(name); err != nil {
			return err
		}
	}
	return ValidateDetect(p.Detect)
}

// Fingerprint hashes the declarative content, independently of installation path.
func Fingerprint(p *Profile) (Dependency, error) {
	copy := *p
	copy.Source, copy.Origin = "", ""
	data, err := json.Marshal(copy)
	if err != nil {
		return Dependency{}, err
	}
	return Dependency{Source: p.Source, SHA256: fmt.Sprintf("%x", sha256.Sum256(data))}, nil
}

// Pin records every transitive dependency, including resolution by alias.
func Pin(p *Profile, registry map[string]*Profile) error {
	reg := make(map[string]*Profile, len(registry)+1)
	for k, v := range registry {
		reg[k] = v
	}
	reg[p.Name] = p
	for _, alias := range p.Aliases {
		reg[alias] = p
	}
	order, err := dependencyOrder(p.Name, reg)
	if err != nil {
		return err
	}
	pins := make(map[string]Dependency)
	for _, name := range order {
		if name == p.Name {
			continue
		}
		pin, err := Fingerprint(reg[name])
		if err != nil {
			return err
		}
		pins[name] = pin
	}
	p.Reviewed = pins
	p.ReviewedPolicy = ""
	resolved, err := ResolveProfile(p.Name, reg, DetectMatchContext())
	if err != nil {
		return err
	}
	p.ReviewedPermissions = PermissionEntries(resolved)
	p.ReviewedPolicy, err = PermissionDigest(resolved)
	return err
}

// CheckReviewed rejects changed, missing or shadowed dependencies.
func CheckReviewed(p *Profile, registry map[string]*Profile) error {
	keys := make([]string, 0, len(p.Reviewed))
	for name := range p.Reviewed {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	for _, name := range keys {
		current, ok := registry[name]
		if !ok {
			return fmt.Errorf("profile %q: reviewed dependency %q is missing", p.Name, name)
		}
		actual, err := Fingerprint(current)
		if err != nil {
			return err
		}
		if actual != p.Reviewed[name] {
			return fmt.Errorf("profile %q: dependency %q changed or was shadowed; run 'bws profile review %s' before approving", p.Name, name, p.Name)
		}
	}
	return nil
}

// Target returns a validated destination without creating directories.
func Target(name, dir string, local bool) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	base := GlobalProfilesDir()
	if local {
		base = LocalProfilesDir(dir)
	}
	return filepath.Join(base, name+".json"), nil
}
