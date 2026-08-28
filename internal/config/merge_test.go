package config

import (
	"testing"
)

func boolPtr(b bool) *bool    { return &b }
func strPtr(s string) *string { return &s }

func TestMergeNilLocal(t *testing.T) {
	global := &Config{SandboxPath: "/home/user/.sandbox"}
	result := Merge(global, nil)
	if result.SandboxPath != "/home/user/.sandbox" {
		t.Errorf("expected SandboxPath %q, got %q", "/home/user/.sandbox", result.SandboxPath)
	}
}

func TestMergeScalarOverride(t *testing.T) {
	global := &Config{
		SandboxPath:     "/home/user/.sandbox",
		ModelsJSONPath:  "~/info/llm/models.json",
		TmuxSessionName: "bwrap-dev",
		MaxFileCount:    1000,
	}
	local := &Config{
		SandboxPath:     "/home/user/.sandbox/custom",
		TmuxSessionName: "custom-session",
		MaxFileCount:    500,
	}
	result := Merge(global, local)
	if result.SandboxPath != "/home/user/.sandbox/custom" {
		t.Errorf("expected SandboxPath %q, got %q", "/home/user/.sandbox/custom", result.SandboxPath)
	}
	if result.TmuxSessionName != "custom-session" {
		t.Errorf("expected TmuxSessionName %q, got %q", "custom-session", result.TmuxSessionName)
	}
	if result.MaxFileCount != 500 {
		t.Errorf("expected MaxFileCount 500, got %d", result.MaxFileCount)
	}
	if result.ModelsJSONPath != "~/info/llm/models.json" {
		t.Errorf("expected ModelsJSONPath preserved as %q, got %q", "~/info/llm/models.json", result.ModelsJSONPath)
	}
}

func TestMergeSystemDeepMerge(t *testing.T) {
	global := &Config{
		System: &SystemConfig{
			ShareNet:   boolPtr(true),
			Clearenv:   boolPtr(true),
			UnshareUTS: boolPtr(false),
			Hostname:   strPtr("bubble"),
		},
	}
	local := &Config{
		System: &SystemConfig{
			ShareNet:   boolPtr(false),
			UnshareUTS: boolPtr(true),
		},
	}
	result := Merge(global, local)
	if result.System == nil {
		t.Fatal("expected System to be non-nil")
	}
	if *result.System.ShareNet != false {
		t.Errorf("expected ShareNet false, got %t", *result.System.ShareNet)
	}
	if *result.System.Clearenv != true {
		t.Errorf("expected Clearenv preserved as true, got %t", *result.System.Clearenv)
	}
	if *result.System.UnshareUTS != true {
		t.Errorf("expected UnshareUTS true, got %t", *result.System.UnshareUTS)
	}
	if *result.System.Hostname != "bubble" {
		t.Errorf("expected Hostname preserved as %q, got %q", "bubble", *result.System.Hostname)
	}
}

func TestMergeFeaturesDeepMerge(t *testing.T) {
	global := &Config{
		Features: &FeaturesConfig{
			EnableSSH:         boolPtr(true),
			EnableX11:         boolPtr(true),
			AutoRepoDeployKey: boolPtr(true),
			SSHKeys:           []string{},
		},
	}
	local := &Config{
		Features: &FeaturesConfig{
			EnableSSH: boolPtr(false),
			SSHKeys:   []string{"/home/user/.ssh/id_ed25519"},
		},
	}
	result := Merge(global, local)
	if result.Features == nil {
		t.Fatal("expected Features to be non-nil")
	}
	if *result.Features.EnableSSH != false {
		t.Errorf("expected EnableSSH false, got %t", *result.Features.EnableSSH)
	}
	if *result.Features.EnableX11 != true {
		t.Errorf("expected EnableX11 preserved as true, got %t", *result.Features.EnableX11)
	}
	if *result.Features.AutoRepoDeployKey != true {
		t.Errorf("expected AutoRepoDeployKey preserved as true, got %t", *result.Features.AutoRepoDeployKey)
	}
	if len(result.Features.SSHKeys) != 1 || result.Features.SSHKeys[0] != "/home/user/.ssh/id_ed25519" {
		t.Errorf("expected SSHKeys overridden with 1 key, got %v", result.Features.SSHKeys)
	}
}

func TestMergeEnvDeepMerge(t *testing.T) {
	global := &Config{
		Env: map[string]string{
			"HOME":   "/home/user",
			"TERM":   "xterm-256color",
			"EDITOR": "emacs -nw",
		},
	}
	local := &Config{
		Env: map[string]string{
			"EDITOR": "vim",
			"LANG":   "C.UTF-8",
		},
	}
	result := Merge(global, local)
	if result.Env["HOME"] != "/home/user" {
		t.Errorf("expected HOME preserved, got %q", result.Env["HOME"])
	}
	if result.Env["TERM"] != "xterm-256color" {
		t.Errorf("expected TERM preserved, got %q", result.Env["TERM"])
	}
	if result.Env["EDITOR"] != "vim" {
		t.Errorf("expected EDITOR overridden to vim, got %q", result.Env["EDITOR"])
	}
	if result.Env["LANG"] != "C.UTF-8" {
		t.Errorf("expected LANG added, got %q", result.Env["LANG"])
	}
}

func TestMergeArraysCombined(t *testing.T) {
	global := &Config{
		Path:    []string{"/usr/bin", "/bin"},
		BindsRW: []BindEntry{{Host: "/home/user/.cargo", Sandbox: "/home/user/.cargo"}},
		BindsRO: []BindEntry{{Host: "/usr", Sandbox: "/usr"}},
		Copy:    []string{"/home/user/bin/prog1"},
	}
	local := &Config{
		Path:    []string{"/usr/local/bin", "/usr/bin"},
		BindsRW: []BindEntry{{Host: "/home/user/custom", Sandbox: "/home/user/custom"}},
		Copy:    []string{"/home/user/bin/prog2"},
	}
	result := Merge(global, local)
	if len(result.Path) != 3 || result.Path[0] != "/usr/bin" || result.Path[1] != "/bin" || result.Path[2] != "/usr/local/bin" {
		t.Errorf("expected Path merged with 3 unique entries, got %v", result.Path)
	}
	if len(result.BindsRW) != 2 || result.BindsRW[0].Host != "/home/user/.cargo" || result.BindsRW[1].Host != "/home/user/custom" {
		t.Errorf("expected BindsRW combined with 2 entries, got %v", result.BindsRW)
	}
	if len(result.BindsRO) != 1 || result.BindsRO[0].Host != "/usr" {
		t.Errorf("expected BindsRO preserved from global, got %v", result.BindsRO)
	}
	if len(result.Copy) != 2 || result.Copy[0] != "/home/user/bin/prog1" || result.Copy[1] != "/home/user/bin/prog2" {
		t.Errorf("expected Copy combined with 2 entries, got %v", result.Copy)
	}
}

func TestMergeNilGlobalFeatures(t *testing.T) {
	global := &Config{}
	local := &Config{
		Features: &FeaturesConfig{
			EnableSSH: boolPtr(true),
		},
	}
	result := Merge(global, local)
	if result.Features == nil {
		t.Fatal("expected Features to be non-nil")
	}
	if *result.Features.EnableSSH != true {
		t.Errorf("expected EnableSSH true, got %t", *result.Features.EnableSSH)
	}
}

func TestMergeNilLocalFeatures(t *testing.T) {
	global := &Config{
		Features: &FeaturesConfig{
			EnableSSH: boolPtr(true),
		},
	}
	local := &Config{}
	result := Merge(global, local)
	if result.Features == nil {
		t.Fatal("expected Features to be non-nil")
	}
	if *result.Features.EnableSSH != true {
		t.Errorf("expected EnableSSH preserved as true, got %t", *result.Features.EnableSSH)
	}
}

func TestMergeNilSystem(t *testing.T) {
	global := &Config{}
	local := &Config{
		System: &SystemConfig{
			Hostname: strPtr("custom"),
		},
	}
	result := Merge(global, local)
	if result.System == nil {
		t.Fatal("expected System to be non-nil")
	}
	if *result.System.Hostname != "custom" {
		t.Errorf("expected Hostname custom, got %q", *result.System.Hostname)
	}
}

func TestMergeEmptyLocal(t *testing.T) {
	global := &Config{
		SandboxPath:  "/home/user/.sandbox",
		MaxFileCount: 1000,
		System: &SystemConfig{
			ShareNet: boolPtr(true),
		},
		Features: &FeaturesConfig{
			EnableSSH: boolPtr(true),
		},
		Env:  map[string]string{"TERM": "xterm-256color"},
		Path: []string{"/usr/bin"},
	}
	local := &Config{}
	result := Merge(global, local)
	if result.SandboxPath != "/home/user/.sandbox" {
		t.Errorf("expected SandboxPath preserved, got %q", result.SandboxPath)
	}
	if result.MaxFileCount != 1000 {
		t.Errorf("expected MaxFileCount preserved, got %d", result.MaxFileCount)
	}
	if *result.System.ShareNet != true {
		t.Errorf("expected ShareNet preserved")
	}
	if *result.Features.EnableSSH != true {
		t.Errorf("expected EnableSSH preserved")
	}
	if result.Env["TERM"] != "xterm-256color" {
		t.Errorf("expected TERM preserved")
	}
	if len(result.Path) != 1 || result.Path[0] != "/usr/bin" {
		t.Errorf("expected Path preserved")
	}
}
