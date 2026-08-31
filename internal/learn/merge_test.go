package learn

import (
	"path/filepath"
	"testing"

	"bws/internal/config"
)

func TestApplyDelta(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.jsonc")

	// Create initial config
	if err := config.CreateDefault(cfgPath); err != nil {
		t.Fatalf("CreateDefault failed: %v", err)
	}

	delta := &Delta{
		Path:       []string{"~/.cargo/bin"},
		BindsRW:    []string{"~/.gemini", "~/.config/mytool"},
		BindsRO:    []string{"~/.custom_ro"},
		UpgradedRO: []string{"~/.gitconfig"}, // default config has ~/.gitconfig in binds_ro
		Features: DetectedFeatures{
			SSH:  true,
			DBus: true,
		},
	}

	mergeRes, err := ApplyDelta(cfgPath, delta)
	if err != nil {
		t.Fatalf("ApplyDelta failed: %v", err)
	}

	if mergeRes.AddedRW != 2 {
		t.Errorf("mergeRes.AddedRW = %d, want 2", mergeRes.AddedRW)
	}
	if mergeRes.AddedRO != 1 {
		t.Errorf("mergeRes.AddedRO = %d, want 1", mergeRes.AddedRO)
	}
	if mergeRes.UpgradedRO != 1 {
		t.Errorf("mergeRes.UpgradedRO = %d, want 1", mergeRes.UpgradedRO)
	}
	if mergeRes.AddedPath != 1 {
		t.Errorf("mergeRes.AddedPath = %d, want 1", mergeRes.AddedPath)
	}
	if len(mergeRes.EnabledFeatures) != 2 {
		t.Errorf("mergeRes.EnabledFeatures = %v, want 2 features", mergeRes.EnabledFeatures)
	}

	// Verify loaded config
	cfg, err := config.LoadFile(cfgPath)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	// Check that ~/.gitconfig was removed from binds_ro
	for _, ro := range cfg.BindsRO {
		if ro.Host == "~/.gitconfig" {
			t.Errorf("expected ~/.gitconfig to be removed from binds_ro")
		}
	}

	// Check path
	foundPath := false
	for _, p := range cfg.Path {
		if p == "~/.cargo/bin" {
			foundPath = true
			break
		}
	}
	if !foundPath {
		t.Errorf("expected ~/.cargo/bin in cfg.Path")
	}

	// Check features
	if cfg.Features == nil || cfg.Features.EnableSSH == nil || !*cfg.Features.EnableSSH {
		t.Errorf("expected enable_ssh = true in config")
	}
	if cfg.Features == nil || cfg.Features.EnableDBus == nil || !*cfg.Features.EnableDBus {
		t.Errorf("expected enable_dbus = true in config")
	}
}
