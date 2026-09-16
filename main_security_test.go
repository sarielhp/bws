package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"bws/internal/config"
)

func TestPlanDoesNotInitializeHome(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	cmd := exec.Command(bwPath, "plan", "--no-ssh", "--no-dbus")
	cmd.Dir = project
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("plan: %v\n%s", err, output)
	}
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("preview wrote to home: %v", entries)
	}
}

func TestLaunchRejectsPlantedConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	project := t.TempDir()
	path := filepath.Join(project, ".bws.jsonc")
	if err := os.WriteFile(path, []byte(`{"binds_rw":["/"]}`), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"plan"}, {"run", "--no-ssh", "--", "true"}} {
		cmd := exec.Command(bwPath, args...)
		cmd.Dir = project
		out, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(out), "untrusted") {
			t.Fatalf("planted config: %v\n%s", err, out)
		}
	}
	cmd := exec.Command(bwPath, "config", "trust")
	cmd.Dir = project
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("trust: %v\n%s", err, out)
	}
	if _, err := config.LoadLocalFile(path); err != nil {
		t.Fatal(err)
	}
}

func TestUnknownProfileStopsLaunch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())
	if err := os.Mkdir(".bws", 0755); err != nil {
		t.Fatal(err)
	}
	if err := config.WriteTrustedFile(".bws/config.jsonc", []byte(`{"profiles":["missing-security-profile"]}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := loadConfigs(false); err == nil {
		t.Fatal("missing profile ignored")
	}
}
