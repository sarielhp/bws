package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"bws/internal/config"
	"bws/internal/detect"
	"bws/internal/profile"
	"bws/internal/util"
)

// Setup screen tabs, in display order. Profiles name their tab through the
// "category" field; profiles without one that only bundle others are not
// shown (their parts are), and any others land in "Other".
var setupTabs = []struct{ category, title string }{
	{"languages", "Languages"},
	{"editors", "Editors"},
	{"agents", "Agents"},
	{"tools", "Tools"},
	{"restrictions", "Restrictions"},
	{"access", "Access"},
	{"other", "Other"},
}

// Access settings are features rather than profiles.
var setupAccess = []struct{ key, label, desc string }{
	{"network", "Network", "Allow network access. Off runs the sandbox in its own empty network namespace."},
	{"push", "Git push", "Forward a per-session SSH agent holding only this repository's GitHub deploy key (or your configured ssh_keys)."},
	{"gui", "GUI windows (X11)", "Expose the X11 display. Note: X11 clients can read and inject keystrokes in other windows."},
}

// personalTabs hold choices that follow the user between projects; the
// languages tab follows the project and comes from detection.
var personalTabs = []string{"editors", "agents", "tools", "restrictions"}

// setupItem is one checkbox.
type setupItem struct {
	category string
	key      string // profile name, or access key
	label    string
	desc     string
	on       bool
	note     string
	prof     *profile.Profile // nil for access items
}

// setupState is the selection independent of any screen.
type setupState struct {
	items    []setupItem
	registry map[string]*profile.Profile
	extras   []string // configured names outside the registry, kept as-is
}

// setupMemory is host-only state: the last selection, used to pre-check
// choices in new workspaces.
type setupMemory struct {
	Profiles []string `json:"profiles"`
	Access   []string `json:"access"`
}

func setupMemoryPath() string { return filepath.Join(config.Dir(), "setup.json") }

func loadSetupMemory() *setupMemory {
	data, err := os.ReadFile(setupMemoryPath())
	if err != nil {
		return nil
	}
	var m setupMemory
	if json.Unmarshal(data, &m) != nil {
		return nil
	}
	return &m
}

func saveSetupMemory(s *setupState) {
	m := setupMemory{Profiles: s.checked(false), Access: s.checked(true)}
	data, err := json.MarshalIndent(m, "", "  ")
	if err == nil && os.MkdirAll(config.Dir(), 0700) == nil {
		_ = os.WriteFile(setupMemoryPath(), append(data, '\n'), 0600)
	}
}

// newSetupState lists every visible profile and access setting. An existing
// configuration decides every checkbox; otherwise detection and the
// remembered selection do.
func newSetupState(registry map[string]*profile.Profile, existing *config.Config, detected []string, mem *setupMemory) *setupState {
	s := &setupState{registry: registry}
	for _, p := range visibleProfiles(registry) {
		item := setupItem{category: p.Category, key: p.Name, label: p.Name, desc: p.Description, prof: p}
		if item.category == "" {
			item.category = "other"
		}
		if bin := profileBinary(p); bin != "" && !util.CommandExists(bin) {
			item.note = "not installed"
		}
		if slices.Contains(detected, p.Name) {
			item.note = "detected"
		}
		s.items = append(s.items, item)
	}
	for _, a := range setupAccess {
		s.items = append(s.items, setupItem{category: "access", key: a.key, label: a.label, desc: a.desc})
	}
	if existing != nil {
		s.applyExisting(existing)
	} else {
		s.applyDefaults(detected, mem)
	}
	return s
}

// visibleProfiles returns profiles in tab order, then by name. A profile
// without a category that only requires others is a bundle and is hidden.
func visibleProfiles(registry map[string]*profile.Profile) []*profile.Profile {
	var out []*profile.Profile
	seen := map[string]bool{}
	for _, p := range registry {
		if seen[p.Name] || setupHidden[p.Name] || (p.Category == "" && isBundle(p)) {
			continue
		}
		seen[p.Name] = true
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		ti, tj := tabIndex(out[i].Category), tabIndex(out[j].Category)
		if ti != tj {
			return ti < tj
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// setupHidden profiles duplicate a setting on the Access tab.
var setupHidden = map[string]bool{"offline": true}

func isBundle(p *profile.Profile) bool {
	return p.Kind == "compound" || (len(p.Requires) > 0 && len(p.Path) == 0 && len(p.Env) == 0 && len(p.PassEnv) == 0 &&
		len(p.BindsRW) == 0 && len(p.BindsRO) == 0 && len(p.Mask) == 0 && !p.UnshareNet && !p.NoNet && p.Features == nil)
}

func tabIndex(category string) int {
	if category == "" {
		category = "other"
	}
	for i, t := range setupTabs {
		if t.category == category {
			return i
		}
	}
	return len(setupTabs) - 1
}

// profileBinary is the executable a profile's version test runs, if any.
func profileBinary(p *profile.Profile) string {
	for _, t := range p.Tests {
		if len(t.Cmd) > 0 && !strings.Contains(t.Cmd[0], "/") {
			return t.Cmd[0]
		}
	}
	return ""
}

func (s *setupState) find(key string) *setupItem {
	for i := range s.items {
		if s.items[i].key == key {
			return &s.items[i]
		}
	}
	return nil
}

// closure expands names through requires, so bundles mark their parts.
func (s *setupState) closure(names []string) map[string]bool {
	out := map[string]bool{}
	var walk func(string)
	walk = func(n string) {
		p := s.registry[n]
		if p == nil || out[p.Name] {
			return
		}
		out[p.Name] = true
		for _, r := range p.Requires {
			walk(r)
		}
	}
	for _, n := range names {
		walk(n)
	}
	return out
}

func (s *setupState) applyExisting(cfg *config.Config) {
	selected := s.closure(cfg.Profiles)
	for _, n := range cfg.Profiles {
		// Keep names the screen cannot show: unknown or hidden profiles.
		if p := s.registry[n]; p == nil || (s.find(p.Name) == nil && !isBundle(p)) {
			s.extras = append(s.extras, n)
		}
	}
	for i := range s.items {
		it := &s.items[i]
		switch it.key {
		case "network":
			it.on = !config.FeatureEnabledDefault(cfg, func(f *config.FeaturesConfig) *bool { return f.NoNet }, false)
		case "push":
			it.on = config.FeatureEnabled(cfg, func(f *config.FeaturesConfig) *bool { return f.EnableSSH })
		case "gui":
			it.on = config.FeatureEnabledDefault(cfg, func(f *config.FeaturesConfig) *bool { return f.EnableX11 }, false)
		default:
			it.on = it.prof != nil && selected[it.key]
		}
	}
}

func (s *setupState) applyDefaults(detected []string, mem *setupMemory) {
	if mem == nil {
		mem = &setupMemory{Access: []string{"network"}}
	}
	langDetected := false
	for i := range s.items {
		it := &s.items[i]
		switch {
		case it.prof == nil:
			it.on = slices.Contains(mem.Access, it.key)
		case slices.Contains(detected, it.key):
			it.on = true
			langDetected = langDetected || it.category == "languages"
		case slices.Contains(personalTabs, it.category):
			it.on = slices.Contains(mem.Profiles, it.key)
		}
	}
	if langDetected {
		return
	}
	for i := range s.items {
		if s.items[i].category == "languages" && slices.Contains(mem.Profiles, s.items[i].key) {
			s.items[i].on = true
		}
	}
	s.checkRequirements()
}

// toggle flips an item; checking a profile also checks what it requires.
func (s *setupState) toggle(key string) {
	it := s.find(key)
	if it == nil {
		return
	}
	it.on = !it.on
	if it.on && it.prof != nil {
		s.checkRequirements()
	}
}

func (s *setupState) checkRequirements() {
	var on []string
	for _, it := range s.items {
		if it.on && it.prof != nil {
			on = append(on, it.key)
		}
	}
	for name := range s.closure(on) {
		if it := s.find(name); it != nil {
			it.on = true
		}
	}
}

// checked returns the checked profile names, or the checked access keys.
func (s *setupState) checked(access bool) []string {
	var out []string
	for _, it := range s.items {
		if it.on && (it.prof == nil) == access {
			out = append(out, it.key)
		}
	}
	return out
}

func (s *setupState) isOn(key string) bool {
	it := s.find(key)
	return it != nil && it.on
}

// profiles is the list to write: checked profiles, then names the registry
// does not know, unchanged.
func (s *setupState) profiles() []string {
	return uniqueNames(append(s.checked(false), s.extras...))
}

func (s *setupState) features() map[string]bool {
	return map[string]bool{
		"no_net":     !s.isOn("network"),
		"enable_ssh": s.isOn("push"),
		"enable_x11": s.isOn("gui"),
	}
}

// selectOnly checks exactly the given profiles (flag form); access settings
// keep their current state.
func (s *setupState) selectOnly(names []string) error {
	for _, n := range names {
		if p := s.registry[n]; p == nil {
			return fmt.Errorf("unknown profile %q (see 'bws profile list')", n)
		}
	}
	selected := s.closure(names)
	for i := range s.items {
		if s.items[i].prof != nil {
			s.items[i].on = selected[s.items[i].key]
		}
	}
	return nil
}

// detectedProfiles returns the profiles whose detection rules clearly match.
func detectedProfiles(root string, registry map[string]*profile.Profile) []string {
	evidence, err := detect.Scan(root)
	if err != nil {
		return nil
	}
	matches, err := profile.Suggestions(evidence, registry, false)
	if err != nil {
		return nil
	}
	var names []string
	for _, m := range matches {
		if m.Rank > 1 {
			names = append(names, m.Name)
		}
	}
	return names
}
