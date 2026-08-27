package profile

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

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
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Requires    []string          `json:"requires,omitempty"`
	Path        []string          `json:"path,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	PassEnv     []string          `json:"pass_env,omitempty"`
	Mask        []string          `json:"mask,omitempty"`
	BindsRW     [][]string        `json:"binds_rw,omitempty"`
	BindsRO     [][]string        `json:"binds_ro,omitempty"`
	Detect      *DetectSpec       `json:"detect,omitempty"`
	Tests       []TestSpec        `json:"tests,omitempty"`
	Rules       []ProfileRule     `json:"rules,omitempty"`
	Source      string            `json:"source,omitempty"` // "embedded", "global", "local"
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
						registry[p.Name] = &p
					}
				}
			}
		}
	}

	// 2. Global profiles (~/.config/bwss/profiles)
	globalDir := GlobalProfilesDir()
	loadDirProfiles(globalDir, "global", registry)

	// 3. Local project profiles (.bws/profiles)
	if projectDir != "" {
		localDir := LocalProfilesDir(projectDir)
		loadDirProfiles(localDir, "local", registry)
	}

	return registry, nil
}

func loadDirProfiles(dir, source string, registry map[string]*Profile) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".jsonc") {
			path := filepath.Join(dir, name)
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			var p Profile
			if err := json.Unmarshal(data, &p); err == nil && p.Name != "" {
				p.Source = source
				registry[p.Name] = &p
			}
		}
	}
}

// ResolveProfile resolves a profile and its full dependency tree without cycles.
func ResolveProfile(name string, registry map[string]*Profile, ctx MatchContext) (*ResolvedProfile, error) {
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
			if n == name {
				return fmt.Errorf("profile %q not found in registry", n)
			}
			// External/system dependency without a separate profile; skip gracefully
			return nil
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

	res := &ResolvedProfile{
		Name:     name,
		Profiles: order,
		Env:      make(map[string]string),
	}

	seenRW := make(map[string]bool)
	seenRO := make(map[string]bool)
	seenPath := make(map[string]bool)
	seenPassEnv := make(map[string]bool)
	seenMask := make(map[string]bool)

	for _, pName := range order {
		p := registry[pName]
		if res.Description == "" && p.Description != "" {
			res.Description = p.Description
		}

		for _, pt := range p.Path {
			if !seenPath[pt] {
				seenPath[pt] = true
				res.Path = append(res.Path, pt)
			}
		}

		for _, pe := range p.PassEnv {
			if !seenPassEnv[pe] {
				seenPassEnv[pe] = true
				res.PassEnv = append(res.PassEnv, pe)
			}
		}

		for _, m := range p.Mask {
			if !seenMask[m] {
				seenMask[m] = true
				res.Mask = append(res.Mask, m)
			}
		}

		for k, v := range p.Env {
			res.Env[k] = v
		}

		for _, b := range p.BindsRW {
			key := strings.Join(b, "->")
			if !seenRW[key] {
				seenRW[key] = true
				res.BindsRW = append(res.BindsRW, b)
			}
		}

		for _, b := range p.BindsRO {
			key := strings.Join(b, "->")
			if !seenRO[key] {
				seenRO[key] = true
				res.BindsRO = append(res.BindsRO, b)
			}
		}

		for _, r := range p.Rules {
			if MatchRule(r, ctx) {
				for _, pt := range r.Path {
					if !seenPath[pt] {
						seenPath[pt] = true
						res.Path = append(res.Path, pt)
					}
				}
				for k, v := range r.Env {
					res.Env[k] = v
				}
				for _, b := range r.BindsRW {
					key := strings.Join(b, "->")
					if !seenRW[key] {
						seenRW[key] = true
						res.BindsRW = append(res.BindsRW, b)
					}
				}
				for _, b := range r.BindsRO {
					key := strings.Join(b, "->")
					if !seenRO[key] {
						seenRO[key] = true
						res.BindsRO = append(res.BindsRO, b)
					}
				}
			}
		}

		res.Tests = append(res.Tests, p.Tests...)
	}

	return res, nil
}

// DetectProfiles checks workspace files against profile detection rules.
func DetectProfiles(projectDir string, registry map[string]*Profile) []*Profile {
	var detected []*Profile
	seen := make(map[string]bool)

	// Collect files in project root (shallow check for fast detection)
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return nil
	}

	var rootFiles []string
	for _, e := range entries {
		rootFiles = append(rootFiles, e.Name())
	}

	dirBase := filepath.Base(projectDir)

	for _, p := range registry {
		if p.Detect == nil {
			continue
		}
		if seen[p.Name] {
			continue
		}

		matched := false

		// Check exact files
		for _, f := range p.Detect.Files {
			for _, rf := range rootFiles {
				if strings.EqualFold(f, rf) {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}

		// Check globs against root files
		if !matched && len(p.Detect.Globs) > 0 {
			for _, g := range p.Detect.Globs {
				for _, rf := range rootFiles {
					if matchGlob(g, rf) {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}
		}

		// Check directory name substrings
		if !matched && len(p.Detect.DirContains) > 0 {
			for _, sub := range p.Detect.DirContains {
				if strings.Contains(strings.ToLower(dirBase), strings.ToLower(sub)) {
					matched = true
					break
				}
			}
		}

		if matched {
			seen[p.Name] = true
			detected = append(detected, p)
		}
	}

	return detected
}

func matchGlob(pattern, name string) bool {
	matched, err := filepath.Match(pattern, name)
	if err != nil {
		return false
	}
	return matched
}
