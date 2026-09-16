package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"bws/internal/cli"
	"bws/internal/config"
	"bws/internal/policy"
	"bws/internal/profile"
)

func compoundCommand(t *testing.T, dir string, args ...string) (string, string, error) {
	t.Helper()
	cmd := exec.Command(bwPath, args...)
	cmd.Dir = dir
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	return out.String(), errOut.String(), err
}

func compoundFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
}

func compoundHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	if err := config.CreateDefault(config.GlobalPath()); err != nil {
		t.Fatal(err)
	}
}

func TestCompoundSaveSuggestAndInit(t *testing.T) {
	compoundHome(t)
	first, second := t.TempDir(), t.TempDir()
	for _, root := range []string{first, second} {
		compoundFile(t, filepath.Join(root, "go.mod"), "module example")
	}
	out, errOut, err := compoundCommand(t, first, "init", "--profile", "go,git", "--no-ssh")
	if err != nil {
		t.Fatalf("init: %v %s %s", err, out, errOut)
	}
	out, errOut, err = compoundCommand(t, first, "profile", "save", "go-agent", "--yes")
	if err != nil {
		t.Fatalf("save: %v %s %s", err, out, errOut)
	}
	out, errOut, err = compoundCommand(t, second, "profile", "suggest", "--compound", "--json")
	if err != nil {
		t.Fatalf("suggest: %v %s", err, errOut)
	}
	var report cli.SuggestionReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range report.Suggestions {
		if s.Name == "go-agent" {
			found = true
		}
	}
	if !found {
		t.Fatalf("saved profile not suggested: %s", out)
	}
	compoundFile(t, filepath.Join(second, "package.json"), "{}")
	out, errOut, err = compoundCommand(t, second, "init", "--profile", "go-agent")
	if err != nil {
		t.Fatalf("second init: %v %s %s", err, out, errOut)
	}
	cfg, err := config.LoadLocalFile(filepath.Join(second, ".bws", "config.jsonc"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.Profiles, []string{"go-agent"}) || len(cfg.BindsRW) > 0 {
		t.Fatal("implicit profiles or duplicated mounts")
	}
	resolved, err := policy.Load(second)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(resolved.Config.Env["GOPATH"], "/.go") || config.FeatureEnabled(resolved.Config, func(f *config.FeaturesConfig) *bool { return f.EnableSSH }) {
		t.Fatal("saved settings not applied")
	}
	before, err := os.ReadFile(config.FindLocalPath(second))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := compoundCommand(t, second, "init"); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(config.FindLocalPath(second))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("existing selection changed")
	}
}

func TestCompoundDryRunsAndAmbiguity(t *testing.T) {
	compoundHome(t)
	root := t.TempDir()
	compoundFile(t, filepath.Join(root, "go.mod"), "module example")
	before := snapshotTree(t, os.Getenv("HOME"))
	out, errOut, err := compoundCommand(t, root, "profile", "save", "preview", "--dry-run")
	if err != nil {
		t.Fatalf("save preview: %v %s", err, errOut)
	}
	var p profile.Profile
	if err := json.Unmarshal([]byte(out), &p); err != nil {
		t.Fatalf("stdout not pure JSON: %s", out)
	}
	if _, _, err := compoundCommand(t, root, "init"); err == nil {
		t.Fatal("noninteractive guessing should require selection")
	}
	if _, _, err := compoundCommand(t, root, "init", "--profile", "go", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := compoundCommand(t, root, "profile", "suggest", "--json"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".bws")); !os.IsNotExist(err) {
		t.Fatal("preview initialized workspace")
	}
	if !reflect.DeepEqual(before, snapshotTree(t, os.Getenv("HOME"))) {
		t.Fatal("preview changed host policy or trust state")
	}
}

func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			result[path] = "directory"
			return nil
		}
		data, err := os.ReadFile(path)
		if err == nil {
			result[path] = string(data)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestCompoundDependencyReview(t *testing.T) {
	compoundHome(t)
	root := t.TempDir()
	toolPath := filepath.Join(profile.GlobalProfilesDir(), "tool.json")
	compoundFile(t, toolPath, `{"name":"tool","env":{"MODE":"first"}}`)
	out, errOut, err := compoundCommand(t, root, "profile", "compose", "stack", "--profiles", "tool", "--match", "go.mod")
	if err != nil {
		t.Fatalf("compose: %v %s %s", err, out, errOut)
	}
	compoundFile(t, toolPath, `{"name":"tool","env":{"MODE":"second"},"binds_rw":[["/opt/new","/opt/new"]]}`)
	if _, _, err := compoundCommand(t, root, "init", "--profile", "stack"); err == nil {
		t.Fatal("accepted stale dependency")
	}
	path := filepath.Join(profile.GlobalProfilesDir(), "stack.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out, errOut, err = compoundCommand(t, root, "profile", "review", "stack")
	if err != nil {
		t.Fatalf("review: %v %s %s", err, out, errOut)
	}
	if !strings.Contains(out, "/opt/new") {
		t.Fatal("review omitted current permission")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("review wrote without --accept")
	}
	if _, errOut, err := compoundCommand(t, root, "profile", "review", "stack", "--accept"); err != nil {
		t.Fatalf("accept: %v %s", err, errOut)
	}
	if _, errOut, err := compoundCommand(t, root, "init", "--profile", "stack"); err != nil {
		t.Fatalf("init after review: %v %s", err, errOut)
	}
}

func TestCompoundUntrustedInputAndBadNames(t *testing.T) {
	compoundHome(t)
	root := t.TempDir()
	for _, name := range []string{"../escape", "/tmp/escape", ".", "bad/name"} {
		if _, _, err := compoundCommand(t, root, "profile", "save", name); err == nil {
			t.Errorf("accepted %q", name)
		}
	}
	compoundFile(t, filepath.Join(root, ".bws", "config.jsonc"), `{"profiles":["go"]}`)
	before := snapshotTree(t, os.Getenv("HOME"))
	if _, _, err := compoundCommand(t, root, "profile", "save", "untrusted"); err == nil {
		t.Fatal("exported untrusted config")
	}
	if !reflect.DeepEqual(before, snapshotTree(t, os.Getenv("HOME"))) {
		t.Fatal("implicitly approved input")
	}
}

func TestRunAutoInitAndNoInit(t *testing.T) {
	compoundHome(t)
	for _, skip := range []bool{false, true} {
		root := t.TempDir()
		compoundFile(t, filepath.Join(root, "go.mod"), "module example")
		args := []string{"run", "--no-ssh"}
		if skip {
			args = append(args, "--no-init")
		}
		args = append(args, "--", "true")
		if out, errOut, err := compoundCommand(t, root, args...); err != nil {
			t.Fatalf("run: %v %s %s", err, out, errOut)
		}
		_, err := os.Stat(filepath.Join(root, ".bws", "config.jsonc"))
		if skip && !os.IsNotExist(err) {
			t.Fatal("--no-init wrote config")
		}
		if !skip && err != nil {
			t.Fatal("run did not initialize")
		}
	}
}
