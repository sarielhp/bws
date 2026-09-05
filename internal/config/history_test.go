package config

import (
	"testing"
)

func TestDefaultHistoryMasksContainsExpectedPaths(t *testing.T) {
	required := []string{
		"~/.bash_history",
		"~/.zsh_history",
		"~/.local/state/bash/history",
		"~/.local/share/fish/fish_history",
		"~/.local/share/powershell/PSReadLine/ConsoleHost_history.txt",
		"~/.python_history",
		"~/.psql_history",
		"~/.mysql_history",
		"~/.sqlite_history",
		"~/.lesshst",
		"~/.viminfo",
	}

	for _, req := range required {
		found := false
		for _, mask := range DefaultHistoryMasks {
			if mask == req {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected DefaultHistoryMasks to contain %s", req)
		}
	}
}

func TestHistoryMaskEnabled(t *testing.T) {
	if !HistoryMaskEnabled(nil) {
		t.Error("expected HistoryMaskEnabled(nil) to be true by default")
	}

	cfg := &Config{}
	if !HistoryMaskEnabled(cfg) {
		t.Error("expected HistoryMaskEnabled(&Config{}) to be true by default")
	}

	f := false
	cfgDisabled := &Config{
		Features: &FeaturesConfig{
			MaskHistory: &f,
		},
	}
	if HistoryMaskEnabled(cfgDisabled) {
		t.Error("expected HistoryMaskEnabled to be false when explicitly disabled")
	}

	tr := true
	cfgEnabled := &Config{
		Features: &FeaturesConfig{
			MaskHistory: &tr,
		},
	}
	if !HistoryMaskEnabled(cfgEnabled) {
		t.Error("expected HistoryMaskEnabled to be true when explicitly enabled")
	}
}
