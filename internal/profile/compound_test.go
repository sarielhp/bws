package profile

import (
	"runtime"
	"strings"
	"testing"

	"bws/internal/config"
	"bws/internal/detect"
)

func TestCompoundDetectionRankingAndGroups(t *testing.T) {
	rule := &DetectSpec{Files: []string{"go.mod"}}
	registry := map[string]*Profile{
		"go-dev":   {Name: "go-dev", Kind: "compound", Detect: rule, Requires: []string{"go"}},
		"go-agent": {Name: "go-agent", Kind: "compound", Detect: rule, Requires: []string{"go", "ai", "git"}},
		"weak":     {Name: "weak", Kind: "compound", Detect: &DetectSpec{Globs: []string{"*.go"}}},
		"multi":    {Name: "multi", Kind: "compound", Detect: &DetectSpec{AllOf: []DetectSpec{{Files: []string{"go.mod"}}, {Files: []string{"package.json"}}}}},
		"hint":     {Name: "hint", Kind: "compound", Detect: &DetectSpec{DirContains: []string{"oc"}}},
	}
	evidence := &detect.Evidence{Root: "/tmp/project", Entries: []detect.Entry{{Path: "go.mod"}, {Path: "main.go"}}}
	for range 20 {
		got, err := Suggestions(evidence, registry, true)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 3 || got[0].Name != "go-agent" || got[1].Name != "go-dev" || got[0].Rank != got[1].Rank || got[2].Name != "weak" {
			t.Fatalf("unstable or incorrectly ranked results: %#v", got)
		}
	}
	evidence.Entries = append(evidence.Entries, detect.Entry{Path: "web/package.json"})
	got, err := Suggestions(evidence, registry, true)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range got {
		if s.Name == "multi" {
			found = true
			if s.Rank != 3 {
				t.Fatal(s)
			}
		}
	}
	if !found {
		t.Fatal("missing mixed-language match")
	}
}

func TestDetectionValidation(t *testing.T) {
	for _, d := range []*DetectSpec{
		{Files: []string{"../escape"}}, {Globs: []string{"["}},
		{AllOf: []DetectSpec{{}}}, {AllOf: []DetectSpec{{AllOf: []DetectSpec{{Files: []string{"x"}}}}}},
	} {
		if err := ValidateDetect(d); err == nil {
			t.Errorf("accepted invalid rule %#v", d)
		}
	}
	e := &detect.Evidence{Entries: []detect.Entry{{Path: "go.mod"}, {Path: "package.json"}, {Path: "scripts/tool.py"}}}
	d := SuggestedDetect(e)
	if d == nil || len(d.AllOf) != 2 {
		t.Fatalf("wrong proposed rules: %#v", d)
	}
}

func TestDependencyFingerprints(t *testing.T) {
	tool := &Profile{Name: "tool", Source: "embedded", Env: map[string]string{"MODE": "one"}}
	p := &Profile{Name: "stack", Kind: "compound", Requires: []string{"tool"}}
	registry := map[string]*Profile{"tool": tool, "stack": p}
	if err := Pin(p, registry); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveProfile("stack", registry, MatchContext{}); err != nil {
		t.Fatal(err)
	}
	tool.Env["MODE"] = "two"
	if _, err := ResolveProfile("stack", registry, MatchContext{}); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("expected changed dependency error: %v", err)
	}
	tool.Env["MODE"] = "one"
	tool.Source = "local"
	if _, err := ResolveProfile("stack", registry, MatchContext{}); err == nil {
		t.Fatal("accepted shadowed dependency")
	}
	p.Requires = []string{"stack"}
	if err := Pin(p, registry); err == nil {
		t.Fatal("accepted self dependency")
	}
}

func TestCompoundConflicts(t *testing.T) {
	yes, no := true, false
	a := &Profile{Name: "a", Env: map[string]string{"MODE": "a"}, Features: &config.FeaturesConfig{EnableSSH: &yes}}
	b := &Profile{Name: "b", Env: map[string]string{"MODE": "b"}, Features: &config.FeaturesConfig{EnableSSH: &no}}
	p := &Profile{Name: "stack", Kind: "compound", Requires: []string{"a", "b"}}
	reg := map[string]*Profile{"a": a, "b": b, "stack": p}
	if _, err := ResolveProfile("stack", reg, MatchContext{}); err == nil {
		t.Fatal("accepted conflicting environment")
	}
	p.Env = map[string]string{"MODE": "chosen"}
	if _, err := ResolveProfile("stack", reg, MatchContext{}); err == nil {
		t.Fatal("accepted conflicting features")
	}
	p.Features = &config.FeaturesConfig{EnableSSH: &no}
	if _, err := ResolveProfile("stack", reg, MatchContext{}); err != nil {
		t.Fatal(err)
	}
	a.BindsRW = [][]string{{"/a", "/dest"}}
	b.BindsRO = [][]string{{"/b", "/dest"}}
	if _, err := ResolveProfile("stack", reg, MatchContext{}); err == nil {
		t.Fatal("accepted conflicting mounts")
	}
}

func TestHostConditionalPermissionReview(t *testing.T) {
	tool := &Profile{Name: "tool", Source: "global", Rules: []ProfileRule{
		{Match: &MatchCondition{Arch: []string{runtime.GOARCH}}, Env: map[string]string{"MODE": "host"}},
	}}
	p := &Profile{Name: "stack", Kind: "compound", Requires: []string{"tool"}}
	registry := map[string]*Profile{"tool": tool, "stack": p}
	if err := Pin(p, registry); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveProfile("stack", registry, DetectMatchContext()); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveProfile("stack", registry, MatchContext{Arch: "other"}); err == nil || !strings.Contains(err.Error(), "effective permissions changed") {
		t.Fatalf("accepted host-rule change: %v", err)
	}
	registry["wrapper"] = &Profile{Name: "wrapper", Requires: []string{"stack"}}
	if _, err := ResolveProfile("wrapper", registry, MatchContext{Arch: "other"}); err == nil {
		t.Fatal("wrapper bypassed saved dependency review")
	}
}

func TestEmbeddedCompoundProfilesResolve(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	registry, err := LoadRegistry("")
	if err != nil {
		t.Fatal(err)
	}
	for name, p := range registry {
		if name != p.Name || p.Kind != "compound" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			if _, err := ResolveProfile(name, registry, DetectMatchContext()); err != nil {
				t.Fatal(err)
			}
		})
	}
}
