package policy

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"bws/internal/config"
	"bws/internal/profile"
)

func exportFixture(t *testing.T) (*Resolution, map[string]*profile.Profile) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	global, err := config.Parse([]byte(config.DefaultConfigTemplate), "defaults")
	if err != nil {
		t.Fatal(err)
	}
	registry, err := profile.LoadRegistry("")
	if err != nil {
		t.Fatal(err)
	}
	local := &config.Config{Profiles: []string{"go", "git"}, Env: map[string]string{"CUSTOM": "value"}}
	cfg := config.Merge(global, local)
	if err := ApplyRegistry(cfg, registry); err != nil {
		t.Fatal(err)
	}
	return &Resolution{Config: cfg, Global: global, Local: local, Workspace: root}, registry
}

func TestExportRoundTripAndPurity(t *testing.T) {
	r, registry := exportFixture(t)
	original := config.Clone(r.Config)
	plan, err := Export(r, registry, ExportOptions{Name: "go-agent"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.Config, original) {
		t.Fatal("export mutated input policy")
	}
	if plan.Profile.Kind != "compound" || len(plan.Profile.Requires) == 0 || len(plan.Profile.Reviewed) == 0 {
		t.Fatal("missing references or approval")
	}
	for _, b := range plan.Profile.BindsRW {
		if strings.Contains(b[0], "go-build") {
			t.Fatal("duplicated dependency mounts")
		}
	}
	if plan.Profile.Env["CUSTOM"] != "value" {
		t.Fatal("lost explicit environment")
	}
	if _, ok := plan.Profile.Env["BWS_ACTIVE_PROFILES"]; ok {
		t.Fatal("exported runtime variable")
	}
	if _, err := os.Stat(config.ConfigDir()); !os.IsNotExist(err) {
		t.Fatal("export wrote host state")
	}
}

func TestExportFlagsSecretsAndLimitations(t *testing.T) {
	r, registry := exportFixture(t)
	Flags{NoSSH: true, NoNet: true}.Apply(r.Config)
	r.Config.SandboxPath = "/opt/staging"
	r.Config.Path = append(r.Config.Path, "/opt/compiler/bin")
	plan, err := Export(r, registry, ExportOptions{Name: "snapshot", Flatten: true})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Profile.Features.EnableSSH == nil || *plan.Profile.Features.EnableSSH || !(*plan.Profile.Features.NoNet) {
		t.Fatal("lost restrictive flags")
	}
	if len(plan.Unsupported) != 1 || plan.Unsupported[0] != "sandbox_path" || len(plan.MachinePaths) != 1 {
		t.Fatalf("missing limitations: %#v", plan)
	}
	r.Config.Env["API_TOKEN"] = "do-not-print-this"
	if _, err := Export(r, registry, ExportOptions{Name: "snapshot"}); err == nil || strings.Contains(err.Error(), "do-not-print-this") {
		t.Fatalf("bad credential handling: %v", err)
	}
}

func TestWorkspacePortabilityAndSelfSave(t *testing.T) {
	r, registry := exportFixture(t)
	path := filepath.Join(r.Workspace, "data")
	r.Config.BindsRW = append(r.Config.BindsRW, config.BindEntry{Host: path, Sandbox: path})
	plan, err := Export(r, registry, ExportOptions{Name: "saved"})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, b := range plan.Profile.BindsRW {
		if b[0] == WorkspaceToken+"/data" {
			found = true
		}
	}
	if !found {
		t.Fatal("workspace path not made portable")
	}
	registry["saved"] = plan.Profile
	r.Config.Profiles = []string{"saved"}
	if _, err := Export(r, registry, ExportOptions{Name: "saved"}); err != nil {
		t.Fatalf("resaving active profile failed: %v", err)
	}
}

func TestResolutionDoesNotMutateSources(t *testing.T) {
	r, registry := exportFixture(t)
	global := config.Clone(r.Global)
	local := config.Clone(r.Local)
	cfg := config.Merge(r.Global, r.Local)
	if err := ApplyRegistry(cfg, registry); err != nil {
		t.Fatal(err)
	}
	cfg.Env["CUSTOM"] = "changed"
	cfg.Path[0] = "changed"
	if !reflect.DeepEqual(global, r.Global) || !reflect.DeepEqual(local, r.Local) {
		t.Fatal("resolution mutated source configuration")
	}
}

func TestSelectedProfileChange(t *testing.T) {
	r, registry := exportFixture(t)
	pins, err := profile.PinSelections(r.Config.Profiles, registry)
	if err != nil {
		t.Fatal(err)
	}
	r.Config.ReviewedProfiles = pins
	registry["go"].Description += "changed"
	if err := ApplyRegistry(r.Config, registry); err == nil {
		t.Fatal("accepted changed project selection")
	}
}

func TestExportReportsAutoInitAndPreservesSSHPaths(t *testing.T) {
	r, registry := exportFixture(t)
	r.Config.Features = &config.FeaturesConfig{AutoInit: "never", SSHKeys: []string{filepath.Join(os.Getenv("HOME"), ".ssh", "id_key")}}
	plan, err := Export(r, registry, ExportOptions{Name: "saved"})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Unsupported) != 1 || plan.Unsupported[0] != "features.auto_init" {
		t.Fatalf("lost omitted field: %v", plan.Unsupported)
	}
	if r.Config.Features.AutoInit != "never" {
		t.Fatal("mutated source features")
	}
	cfg := &config.Config{Features: plan.Profile.Features}
	config.ExpandWorkspace(cfg, r.Workspace)
	if cfg.Features.SSHKeys[0] != filepath.Join(os.Getenv("HOME"), ".ssh", "id_key") {
		t.Fatal("SSH key token not expanded")
	}
}

func TestRestrictiveProfilesSurviveOldInitDefaults(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	yes := true
	local := &config.Config{Profiles: []string{"no-ssh", "offline"}, Features: &config.FeaturesConfig{EnableSSH: &yes}}
	cfg, err := Resolve(&config.Config{}, local, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Features.EnableSSH == nil || *cfg.Features.EnableSSH {
		t.Fatal("legacy init default re-enabled SSH")
	}
	if cfg.Features.NoNet == nil || !*cfg.Features.NoNet {
		t.Fatal("lost network restriction")
	}
}
