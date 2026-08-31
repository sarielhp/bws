package learn

import (
	"bws/internal/config"
	"testing"
)

func TestComputeDelta_HierarchyAndUpgrades(t *testing.T) {
	homeDir := "/home/testuser"
	trueVal := true

	existingConfig := &config.Config{
		Path: []string{"~/.local/bin", "/usr/bin"},
		BindsRW: []config.BindEntry{
			{Host: "~/.cache/uv"},
			{Host: "/opt/global-data"},
		},
		BindsRO: []config.BindEntry{
			{Host: "~/.config/mytool"},
			{Host: "~/.gitconfig"},
		},
		Features: &config.FeaturesConfig{
			EnableSSH: &trueVal,
		},
	}

	res := &TraceResult{
		Command: []string{"mytool", "build"},
		Features: DetectedFeatures{
			SSH:  true, // already enabled
			DBus: true, // newly discovered
		},
		DiscoveredPath: "~/.cargo/bin", // newly discovered
		BindsRW: []string{
			"~/.cache/uv/wheels", // covered by ~/.cache/uv -> skip
			"~/.config/mytool",   // was RO in config -> UPGRADE to RW!
			"~/.gemini",          // new RW
		},
		BindsRO: []string{
			"~/.gitconfig",          // already in RO -> skip
			"/opt/global-data/conf", // covered by RW /opt/global-data -> skip
			"~/.gemini/settings",    // covered by new RW ~/.gemini -> skip
			"~/.ssh/known_hosts",    // new RO
		},
	}

	delta := ComputeDelta(res, existingConfig, homeDir)

	// PATH check
	if len(delta.Path) != 1 || delta.Path[0] != "~/.cargo/bin" {
		t.Errorf("delta.Path = %v, want [~/.cargo/bin]", delta.Path)
	}

	// BindsRW check
	wantRW := []string{"~/.config/mytool", "~/.gemini"}
	if len(delta.BindsRW) != len(wantRW) {
		t.Fatalf("delta.BindsRW = %v, want %v", delta.BindsRW, wantRW)
	}
	for i := range wantRW {
		if delta.BindsRW[i] != wantRW[i] {
			t.Errorf("delta.BindsRW[%d] = %q, want %q", i, delta.BindsRW[i], wantRW[i])
		}
	}

	// UpgradedRO check
	if len(delta.UpgradedRO) != 1 || delta.UpgradedRO[0] != "~/.config/mytool" {
		t.Errorf("delta.UpgradedRO = %v, want [~/.config/mytool]", delta.UpgradedRO)
	}

	// BindsRO check
	if len(delta.BindsRO) != 1 || delta.BindsRO[0] != "~/.ssh/known_hosts" {
		t.Errorf("delta.BindsRO = %v, want [~/.ssh/known_hosts]", delta.BindsRO)
	}

	// Features check
	if delta.Features.SSH {
		t.Errorf("expected SSH to NOT be in delta because it was already enabled")
	}
	if !delta.Features.DBus {
		t.Errorf("expected DBus to be true in delta")
	}
}

func TestComputeDelta_EmptyWhenCovered(t *testing.T) {
	homeDir := "/home/testuser"
	trueVal := true

	existingConfig := &config.Config{
		Path: []string{"/usr/bin", "~/.local/bin"},
		BindsRW: []config.BindEntry{
			{Host: "~/.config/app"},
		},
		BindsRO: []config.BindEntry{
			{Host: "~/.gitconfig"},
		},
		Features: &config.FeaturesConfig{
			EnableSSH: &trueVal,
		},
	}

	res := &TraceResult{
		Command:        []string{"app"},
		DiscoveredPath: "/usr/bin", // default system path
		Features: DetectedFeatures{
			SSH: true, // already in config
		},
		BindsRW: []string{"~/.config/app/sub"}, // covered by ~/.config/app
		BindsRO: []string{"~/.gitconfig"},      // covered by ~/.gitconfig
	}

	delta := ComputeDelta(res, existingConfig, homeDir)
	if !delta.IsEmpty() {
		t.Errorf("expected delta.IsEmpty() = true, got %+v", delta)
	}
}
