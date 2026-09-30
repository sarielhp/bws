package bwrap

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"bws/internal/config"
)

func TestClearenvDefaultsOn(t *testing.T) {
	off := false
	cases := []struct {
		name string
		cfg  *config.Config
		want bool
	}{
		{"no system block", &config.Config{PassEnv: []string{"USER"}}, true},
		{"system without clearenv", &config.Config{System: &config.SystemConfig{}}, true},
		{"explicit false", &config.Config{System: &config.SystemConfig{Clearenv: &off}}, false},
	}
	for _, tc := range cases {
		var args []string
		addSystemAndNetArgs(tc.cfg, &args, false)
		if got := slices.Contains(args, "--clearenv"); got != tc.want {
			t.Errorf("%s: --clearenv present=%v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestMaskPathsAreAbsolute(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.WriteFile(filepath.Join(cwd, "--unshare-all"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Mask: []string{"--unshare-all"}}
	var args []string
	addMaskArgs(&args, cfg, home, cwd, false)
	want := filepath.Join(cwd, "--unshare-all")
	if !slices.Contains(args, want) || slices.Contains(args, "--unshare-all") {
		t.Fatalf("relative mask not resolved against cwd: %v", args)
	}
}

func TestSandboxTmpArgsNeverPredictable(t *testing.T) {
	args := sandboxTmpArgs(false)
	if slices.Contains(args, "/tmp/bws/SANDBOX_TMP") {
		t.Fatalf("real run used the fixed fallback path: %v", args)
	}
}
