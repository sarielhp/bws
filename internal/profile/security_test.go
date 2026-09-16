package profile

import (
	"os"
	"path/filepath"
	"testing"

	"bws/internal/config"
)

func TestSecurityProfilesDisableSSH(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	registry, err := LoadRegistry(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"no-ssh", "secure-agent"} {
		resolved, err := ResolveProfile(name, registry, MatchContext{})
		if err != nil {
			t.Fatal(err)
		}
		if config.FeatureEnabled(&config.Config{Features: resolved.Features}, func(f *config.FeaturesConfig) *bool { return f.EnableSSH }) {
			t.Errorf("%s enables SSH", name)
		}
	}
}

func TestProfilesFailClosed(t *testing.T) {
	for _, requires := range [][]string{{"missing"}, {"self"}} {
		registry := map[string]*Profile{"self": {Name: "self", Requires: requires}}
		if _, err := ResolveProfile("self", registry, MatchContext{}); err == nil {
			t.Fatal("invalid dependencies accepted")
		}
	}
	registry := map[string]*Profile{"bad": {Name: "bad", BindsRW: [][]string{{"/"}}}}
	if _, err := ResolveProfile("bad", registry, MatchContext{}); err == nil {
		t.Fatal("invalid bind accepted")
	}
}

func TestLocalProfileTrustAndJSONC(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	project := t.TempDir()
	dir := filepath.Join(project, ".bws", "profiles")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "no-ssh.jsonc")
	data := []byte("{ // custom profile\n\"name\":\"no-ssh\",\"features\":{\"enable_ssh\":false}}")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRegistry(project); err == nil {
		t.Fatal("untrusted profile accepted")
	}
	if err := config.TrustWorkspace(project); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRegistry(project); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{broken"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := config.TrustFile(path); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRegistry(project); err == nil {
		t.Fatal("malformed profile ignored")
	}
}
