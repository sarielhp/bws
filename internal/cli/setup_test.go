package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"bws/internal/config"
	"bws/internal/profile"

	tea "github.com/charmbracelet/bubbletea"
)

func embeddedRegistry(t *testing.T) map[string]*profile.Profile {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	reg, err := profile.LoadRegistry(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return reg
}

func onKeys(s *setupState) []string {
	var keys []string
	for _, it := range s.items {
		if it.on {
			keys = append(keys, it.key)
		}
	}
	return keys
}

func TestSetupHidesBundlesAndShowsParts(t *testing.T) {
	s := newSetupState(embeddedRegistry(t), nil, nil, nil)
	for _, hidden := range []string{"go-dev", "latex-use", "ai", "secure-agent", "offline", "no-net"} {
		if s.find(hidden) != nil {
			t.Errorf("bundle %s is listed", hidden)
		}
	}
	for key, category := range map[string]string{"emacs": "editors", "codex": "agents", "latex": "languages", "network": "access"} {
		if it := s.find(key); it == nil || it.category != category {
			t.Errorf("%s: got %+v, want category %s", key, it, category)
		}
	}
}

func TestSetupDefaultsFromDetectionAndMemory(t *testing.T) {
	mem := &setupMemory{Profiles: []string{"emacs", "claude", "codex", "go"}, Access: []string{"network"}}
	s := newSetupState(embeddedRegistry(t), nil, []string{"latex"}, mem)
	on := onKeys(s)
	for _, want := range []string{"latex", "emacs", "claude", "codex", "network"} {
		if !slices.Contains(on, want) {
			t.Errorf("%s not pre-checked: %v", want, on)
		}
	}
	// A detected language wins over remembered languages.
	if slices.Contains(on, "go") {
		t.Errorf("remembered language checked despite detection: %v", on)
	}
	s = newSetupState(embeddedRegistry(t), nil, nil, mem)
	if !s.isOn("go") {
		t.Error("remembered language not used when nothing is detected")
	}
}

func TestSetupExistingConfigExpandsBundlesAndKeepsExtras(t *testing.T) {
	yes := true
	cfg := &config.Config{
		Profiles: []string{"go-dev", "offline", "my-private-tool"},
		Features: &config.FeaturesConfig{EnableX11: &yes},
	}
	s := newSetupState(embeddedRegistry(t), cfg, nil, nil)
	for _, want := range []string{"go", "gopls", "git", "emacs", "gui"} {
		if !s.isOn(want) {
			t.Errorf("%s not checked from go-dev bundle: %v", want, onKeys(s))
		}
	}
	got := s.profiles()
	for _, want := range []string{"offline", "my-private-tool", "go", "gopls"} {
		if !slices.Contains(got, want) {
			t.Errorf("profiles() = %v, missing %s", got, want)
		}
	}
	if slices.Contains(got, "go-dev") {
		t.Errorf("bundle written instead of its parts: %v", got)
	}
}

func TestSetupToggleChecksRequirements(t *testing.T) {
	s := newSetupState(embeddedRegistry(t), &config.Config{}, nil, nil)
	s.toggle("gopls")
	if !s.isOn("go") {
		t.Error("checking gopls did not check go")
	}
	s.toggle("gopls")
	if s.isOn("gopls") || !s.isOn("go") {
		t.Error("unchecking gopls should leave go alone")
	}
}

func TestSetupFeatures(t *testing.T) {
	s := newSetupState(embeddedRegistry(t), &config.Config{}, nil, nil)
	s.find("network").on, s.find("push").on, s.find("gui").on = false, true, false
	f := s.features()
	if !f["no_net"] || !f["enable_ssh"] || f["enable_x11"] {
		t.Errorf("features = %v", f)
	}
}

func key(k string) tea.KeyMsg {
	switch k {
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
}

func send(m *setupTUI, keys ...string) {
	for _, k := range keys {
		m.Update(key(k))
	}
}

func TestSetupTUIKeys(t *testing.T) {
	m := newSetupTUI("t", newSetupState(embeddedRegistry(t), &config.Config{}, nil, nil))
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})

	send(m, "/", "codex", "enter")
	if it := m.current(); it == nil || it.key != "codex" {
		t.Fatalf("search did not put codex first: %+v", it)
	}
	send(m, "space", "esc")
	if !m.s.isOn("codex") || m.filter != "" || m.done {
		t.Fatalf("toggle in search or esc to clear failed: on=%v filter=%q done=%v", m.s.isOn("codex"), m.filter, m.done)
	}

	send(m, "right")
	if setupTabs[m.tabs[m.tab]].category != "editors" {
		t.Fatalf("right did not move to editors: %s", setupTabs[m.tabs[m.tab]].category)
	}
	if view := m.View(); !strings.Contains(view, "emacs") || !strings.Contains(view, "Selected: codex") {
		t.Errorf("view missing editors list or summary:\n%s", view)
	}

	send(m, "enter")
	if !m.done || !m.save {
		t.Fatal("enter did not save")
	}
}

func TestSetupTUICancel(t *testing.T) {
	m := newSetupTUI("t", newSetupState(embeddedRegistry(t), &config.Config{}, nil, nil))
	send(m, "q")
	if !m.done || m.save {
		t.Fatal("q should quit without saving")
	}
}

func TestSetupTUIViewFitsSmallTerminal(t *testing.T) {
	m := newSetupTUI("t", newSetupState(embeddedRegistry(t), &config.Config{}, nil, nil))
	send(m, "right", "right") // agents: long details
	for _, size := range [][2]int{{120, 24}, {80, 24}, {60, 20}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		lines := strings.Split(m.View(), "\n")
		if len(lines) > size[1] {
			t.Errorf("%dx%d: view is %d lines", size[0], size[1], len(lines))
		}
	}
}

// TestRunSetupWithWritesTrustedConfig covers the non-interactive form and
// that a re-run keeps settings the screen does not manage.
func TestRunSetupWithWritesTrustedConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	ws := filepath.Join(home, "paper")
	if err := os.MkdirAll(ws, 0755); err != nil {
		t.Fatal(err)
	}
	if err := RunSetup(SetupOptions{TargetDir: ws, With: []string{"latex", "emacs", "claude", "codex"}}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(ws, ".bws", "config.jsonc")
	cfg, err := config.LoadLocalFile(path)
	if err != nil {
		t.Fatalf("written config is not trusted: %v", err)
	}
	if !slices.Equal(cfg.Profiles, []string{"latex", "emacs", "claude", "codex"}) {
		t.Errorf("profiles = %v", cfg.Profiles)
	}

	if err = config.SetConfigKV(path, "tmux_session_name", "paper"); err != nil {
		t.Fatal(err)
	}
	if err = RunSetup(SetupOptions{TargetDir: ws, With: []string{"latex", "codex"}}); err != nil {
		t.Fatal(err)
	}
	cfg, err = config.LoadLocalFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TmuxSessionName != "paper" || !slices.Equal(cfg.Profiles, []string{"latex", "codex"}) {
		t.Errorf("re-run lost settings or profiles: session=%q profiles=%v", cfg.TmuxSessionName, cfg.Profiles)
	}
}
