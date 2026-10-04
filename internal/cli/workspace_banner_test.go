package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setChain(t *testing.T, home string, dirs ...string) {
	t.Helper()
	t.Setenv("HOME", home)
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(d, ".bws"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, ".bws", "config.jsonc"), []byte("{}\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func captureBanner(t *testing.T, colorEnabled bool) string {
	t.Helper()
	var buf bytes.Buffer
	old := workspaceBannerWriter
	workspaceBannerWriter = &buf
	defer func() { workspaceBannerWriter = old }()
	PrintWorkspaceBanner(colorEnabled)
	return buf.String()
}

func TestBannerMarksAncestorTarget(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "proj")
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	setChain(t, home, root) // only the ancestor has a config

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(sub); err != nil {
		t.Fatal(err)
	}

	out := captureBanner(t, false)
	if !strings.Contains(out, ">>>") {
		t.Fatalf("no target marker:\n%s", out)
	}
	if !strings.Contains(out, "~/proj") {
		t.Fatalf("ancestor not shown ~-relative:\n%s", out)
	}
	if !strings.Contains(out, "shadowed") && !strings.Contains(out, "target") {
		t.Fatalf("no role annotation:\n%s", out)
	}
	// The target line must be the ancestor, not cwd.
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, ">>>") && !strings.Contains(line, "~/proj") {
			t.Fatalf("target marker on wrong directory:\n%s", out)
		}
	}
}

func TestBannerSingleTargetCompact(t *testing.T) {
	home := t.TempDir()
	proj := filepath.Join(home, "proj")
	setChain(t, home, proj)

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(proj); err != nil {
		t.Fatal(err)
	}

	out := captureBanner(t, false)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected one compact line, got:\n%s", out)
	}
	if !strings.Contains(lines[0], ">>>") || !strings.Contains(lines[0], "~/proj") {
		t.Fatalf("compact line wrong: %q", lines[0])
	}
}

func TestBannerSuppressedByEnv(t *testing.T) {
	home := t.TempDir()
	proj := filepath.Join(home, "proj")
	setChain(t, home, proj)

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(proj); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BWS_NO_BANNER", "1")

	if out := captureBanner(t, false); out != "" {
		t.Fatalf("banner not suppressed: %q", out)
	}
}
