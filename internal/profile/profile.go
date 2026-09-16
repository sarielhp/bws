package profile

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tailscale/hujson"

	"bws/internal/config"
	"bws/internal/util"
)

// TestSpec defines a verification or smoke test command.
type TestSpec struct {
	Name     string   `json:"name,omitempty"`
	Cmd      []string `json:"cmd"`
	Type     string   `json:"type,omitempty"`     // "version", "smoke"
	Optional bool     `json:"optional,omitempty"` // auto-skip if host lacks binary
}

// DetectSpec defines heuristics for auto-detecting when a profile applies to a workspace.
type DetectSpec struct {
	Files       []string `json:"files,omitempty"`
	Globs       []string `json:"globs,omitempty"`
	DirContains []string `json:"dir_contains,omitempty"`
}

// Profile represents a declarative sandbox tool capability profile.
type Profile struct {
	Name        string                 `json:"name"`
	Aliases     []string               `json:"aliases,omitempty"`
	Description string                 `json:"description,omitempty"`
	Requires    []string               `json:"requires,omitempty"`
	Path        []string               `json:"path,omitempty"`
	Env         map[string]string      `json:"env,omitempty"`
	PassEnv     []string               `json:"pass_env,omitempty"`
	Mask        []string               `json:"mask,omitempty"`
	BindsRW     [][]string             `json:"binds_rw,omitempty"`
	BindsRO     [][]string             `json:"binds_ro,omitempty"`
	Copy        []string               `json:"copy,omitempty"`
	UnshareNet  bool                   `json:"unshare_net,omitempty"`
	NoNet       bool                   `json:"no_net,omitempty"`
	Features    *config.FeaturesConfig `json:"features,omitempty"`
	Detect      *DetectSpec            `json:"detect,omitempty"`
	Tests       []TestSpec             `json:"tests,omitempty"`
	Rules       []ProfileRule          `json:"rules,omitempty"`
	Source      string                 `json:"source,omitempty"` // "embedded", "global", "local"
}

// ResolvedProfile contains flattened and merged configuration after dependency resolution.
type ResolvedProfile struct {
	Name        string
	Description string
	Profiles    []string
	Path        []string
	Env         map[string]string
	PassEnv     []string
	Mask        []string
	BindsRW     [][]string
	BindsRO     [][]string
	Copy        []string
	UnshareNet  bool
	Features    *config.FeaturesConfig
	Tests       []TestSpec
}

// GlobalProfilesDir returns ~/.config/bws/profiles
func GlobalProfilesDir() string {
	return filepath.Join(util.HomeDir(), ".config", "bws", "profiles")
}

// LocalProfilesDir returns .bws/profiles in the given project directory.
func LocalProfilesDir(projectDir string) string {
	root, _ := config.FindWorkspaceRoot(projectDir)
	return filepath.Join(root, ".bws", "profiles")
}

// LoadRegistry loads all embedded, global, and local project profiles.
func LoadRegistry(projectDir string) (map[string]*Profile, error) {
	registry := make(map[string]*Profile)

	// 1. Embedded profiles
	embeddedEntries, err := fs.ReadDir(embeddedProfilesFS, "embedded_profiles")
	if err == nil {
		for _, entry := range embeddedEntries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
				data, err := embeddedProfilesFS.ReadFile("embedded_profiles/" + entry.Name())
				if err == nil {
					var p Profile
					if err := json.Unmarshal(data, &p); err == nil && p.Name != "" {
						p.Source = "embedded"
						pCopy := p
						registry[p.Name] = &pCopy
						for _, alias := range p.Aliases {
							registry[alias] = &pCopy
						}
					}
				}
			}
		}
	}

	// 2. Global profiles (~/.config/bws/profiles)
	globalDir := GlobalProfilesDir()
	if err := loadDirProfiles(globalDir, "global", registry); err != nil {
		return nil, err
	}

	// 3. Local project profiles (.bws/profiles)
	if projectDir != "" {
		localDir := LocalProfilesDir(projectDir)
		if err := loadDirProfiles(localDir, "local", registry); err != nil {
			return nil, err
		}
	}

	return registry, nil
}

func loadDirProfiles(dir, source string, registry map[string]*Profile) error {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".jsonc") {
			path := filepath.Join(dir, name)
			data, err := os.ReadFile(path)
			if source == "local" && err == nil {
				data, err = config.ReadTrustedFile(path)
			}
			if err != nil {
				return err
			}
			data, err = hujson.Standardize(data)
			if err != nil {
				return fmt.Errorf("profile %s: %w", path, err)
			}
			var p Profile
			if err := json.Unmarshal(data, &p); err != nil {
				return fmt.Errorf("profile %s: %w", path, err)
			}
			if p.Name == "" {
				return fmt.Errorf("profile %s has no name", path)
			}
			p.Source = source
			registry[p.Name] = &p
			for _, alias := range p.Aliases {
				registry[alias] = &p
			}
		}
	}
	return nil
}

func dependencyOrder(name string, registry map[string]*Profile) ([]string, error) {
	visited := make(map[string]bool)
	inStack := make(map[string]bool)
	var order []string

	var visit func(n string) error
	visit = func(n string) error {
		if inStack[n] {
			return fmt.Errorf("cyclic profile dependency detected: %s", n)
		}
		if visited[n] {
			return nil
		}
		p, ok := registry[n]
		if !ok {
			return fmt.Errorf("profile %q not found in registry (required by %q)", n, name)
		}
		inStack[n] = true
		for _, req := range p.Requires {
			if err := visit(req); err != nil {
				return err
			}
		}
		inStack[n] = false
		visited[n] = true
		order = append(order, n)
		return nil
	}

	if err := visit(name); err != nil {
		return nil, err
	}
	return order, nil
}

// ResolveProfile resolves a profile and all required dependencies, failing closed.
func ResolveProfile(name string, registry map[string]*Profile, ctx MatchContext) (*ResolvedProfile, error) {
	order, err := dependencyOrder(name, registry)
	if err != nil {
		return nil, err
	}
	res := &ResolvedProfile{
		Name:     name,
		Profiles: order,
		Env:      make(map[string]string),
	}

	for _, pName := range order {
		p := registry[pName]
		if res.Description == "" && p.Description != "" {
			res.Description = p.Description
		}
		if p.UnshareNet || p.NoNet {
			res.UnshareNet = true
		}
		if p.Features != nil {
			res.Features = config.MergeFeatures(res.Features, p.Features)
		}

		if err := mergeProfileValues(res, p); err != nil {
			return nil, err
		}
		for _, r := range p.Rules {
			if MatchRule(r, ctx) {
				if err := mergeProfileValues(res, &Profile{Name: pName, Path: r.Path, Env: r.Env, BindsRW: r.BindsRW, BindsRO: r.BindsRO}); err != nil {
					return nil, err
				}
			}
		}

		res.Tests = append(res.Tests, p.Tests...)
	}

	return res, nil
}

func appendUniqueStrings(dst *[]string, values []string) {
	for _, value := range values {
		if !slices.Contains(*dst, value) {
			*dst = append(*dst, value)
		}
	}
}

func appendProfileBinds(dst *[][]string, values [][]string, name string) error {
	for _, value := range values {
		if len(value) != 2 || value[0] == "" || value[1] == "" {
			return fmt.Errorf("profile %q requires [host, sandbox] bind pairs", name)
		}
		if !slices.ContainsFunc(*dst, func(b []string) bool { return slices.Equal(b, value) }) {
			*dst = append(*dst, value)
		}
	}
	return nil
}

func mergeProfileValues(res *ResolvedProfile, p *Profile) error {
	appendUniqueStrings(&res.Path, p.Path)
	appendUniqueStrings(&res.PassEnv, p.PassEnv)
	appendUniqueStrings(&res.Mask, p.Mask)
	appendUniqueStrings(&res.Copy, p.Copy)
	for k, v := range p.Env {
		res.Env[k] = v
	}
	if err := appendProfileBinds(&res.BindsRW, p.BindsRW, p.Name); err != nil {
		return err
	}
	return appendProfileBinds(&res.BindsRO, p.BindsRO, p.Name)
}

// DetectProfiles checks workspace files against profile detection rules.
