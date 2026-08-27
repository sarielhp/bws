package profile

import (
	"testing"
)

func TestLoadRegistry(t *testing.T) {
	reg, err := LoadRegistry("")
	if err != nil {
		t.Fatalf("LoadRegistry failed: %v", err)
	}

	required := []string{"go", "python", "rust", "latex", "opencode", "oc", "emacs", "pandoc", "jq"}
	for _, name := range required {
		if _, ok := reg[name]; !ok {
			t.Errorf("expected embedded profile %q to be present", name)
		}
	}
}

func TestResolveProfileDependencies(t *testing.T) {
	reg := map[string]*Profile{
		"base": {
			Name:    "base",
			Path:    []string{"/base/bin"},
			BindsRW: [][]string{{"~/.base", "@@HOME@@/.base"}},
		},
		"child": {
			Name:     "child",
			Requires: []string{"base"},
			Path:     []string{"/child/bin"},
			BindsRW:  [][]string{{"~/.child", "@@HOME@@/.child"}},
		},
	}

	ctx := MatchContext{OS: "linux", Arch: "amd64"}
	resolved, err := ResolveProfile("child", reg, ctx)
	if err != nil {
		t.Fatalf("ResolveProfile failed: %v", err)
	}

	if len(resolved.Profiles) != 2 || resolved.Profiles[0] != "base" || resolved.Profiles[1] != "child" {
		t.Errorf("unexpected resolution order: %v", resolved.Profiles)
	}

	if len(resolved.BindsRW) != 2 {
		t.Errorf("expected 2 binds_rw, got %d", len(resolved.BindsRW))
	}
}

func TestResolveProfileCycle(t *testing.T) {
	reg := map[string]*Profile{
		"a": {Name: "a", Requires: []string{"b"}},
		"b": {Name: "b", Requires: []string{"a"}},
	}

	ctx := MatchContext{OS: "linux"}
	_, err := ResolveProfile("a", reg, ctx)
	if err == nil {
		t.Fatal("expected error on cyclic dependency, got nil")
	}
}

func TestMatchRule(t *testing.T) {
	wslTrue := true
	wslFalse := false

	ruleWSL := ProfileRule{
		Match: &MatchCondition{WSL: &wslTrue, Distro: []string{"ubuntu"}},
	}

	ctxWSL := MatchContext{OS: "linux", Distro: "ubuntu", WSL: true}
	if !MatchRule(ruleWSL, ctxWSL) {
		t.Errorf("expected rule to match on Ubuntu WSL")
	}

	ctxNative := MatchContext{OS: "linux", Distro: "ubuntu", WSL: false}
	if MatchRule(ruleWSL, ctxNative) {
		t.Errorf("expected rule NOT to match on native Linux")
	}

	ruleNonWSL := ProfileRule{
		Match: &MatchCondition{WSL: &wslFalse},
	}
	if MatchRule(ruleNonWSL, ctxWSL) {
		t.Errorf("expected non-WSL rule NOT to match on WSL")
	}
}

func TestConvertFirejailPath(t *testing.T) {
	p := convertFirejailPath("${HOME}/.config/foo")
	if len(p) != 2 || p[0] != "~/.config/foo" || p[1] != "@@HOME@@/.config/foo" {
		t.Errorf("unexpected conversion: %v", p)
	}

	sysP := convertFirejailPath("/var/lib/test")
	if len(sysP) != 2 || sysP[0] != "/var/lib/test" || sysP[1] != "/var/lib/test" {
		t.Errorf("unexpected system conversion: %v", sysP)
	}
}
