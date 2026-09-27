package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"bws/internal/config"
	"bws/internal/stack"
)

func stackTestCommand(t *testing.T, dir string, args ...string) (string, string, error) {
	t.Helper()
	cmd := exec.Command(bwPath, args...)
	cmd.Dir = dir
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	return out.String(), errOut.String(), err
}

func stackTestHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	if err := config.CreateDefault(config.GlobalPath()); err != nil {
		t.Fatal(err)
	}
}

func TestStackListAndShow(t *testing.T) {
	stackTestHome(t)
	dir := t.TempDir()

	out, errOut, err := stackTestCommand(t, dir, "stack", "list")
	if err != nil {
		t.Fatalf("bws stack list failed: %v\n%s\n%s", err, out, errOut)
	}
	for _, expected := range []string{"go-agent", "latex-review", "python-uv", "rust-dev"} {
		if !strings.Contains(out, expected) {
			t.Errorf("expected stack list to contain %q, got:\n%s", expected, out)
		}
	}

	jsonOut, _, err := stackTestCommand(t, dir, "stack", "list", "--json")
	if err != nil {
		t.Fatalf("bws stack list --json failed: %v", err)
	}
	var stacks []*stack.Stack
	if err := json.Unmarshal([]byte(jsonOut), &stacks); err != nil {
		t.Fatalf("bws stack list --json output invalid: %v\n%s", err, jsonOut)
	}
	if len(stacks) < 4 {
		t.Errorf("expected at least 4 stacks in JSON, got %d", len(stacks))
	}

	catOut, _, err := stackTestCommand(t, dir, "stack", "list", "-c", "runtime")
	if err != nil {
		t.Fatalf("bws stack list -c runtime failed: %v", err)
	}
	if !strings.Contains(catOut, "python-uv") || !strings.Contains(catOut, "rust-dev") {
		t.Errorf("expected runtime category to contain python-uv and rust-dev, got:\n%s", catOut)
	}

	showOut, _, err := stackTestCommand(t, dir, "stack", "show", "go-agent")
	if err != nil {
		t.Fatalf("bws stack show go-agent failed: %v", err)
	}
	for _, expected := range []string{"Stack:        go-agent", "Go Agent Persona", "go-dev, secure-agent"} {
		if !strings.Contains(showOut, expected) {
			t.Errorf("expected stack show to contain %q, got:\n%s", expected, showOut)
		}
	}
}

func TestInitWithStack(t *testing.T) {
	stackTestHome(t)
	dir := t.TempDir()

	out, errOut, err := stackTestCommand(t, dir, "init", "--stack", "go-agent", "-y")
	if err != nil {
		t.Fatalf("bws init --stack go-agent failed: %v\n%s\n%s", err, out, errOut)
	}

	cfgPath := filepath.Join(dir, ".bws", "config.jsonc")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("failed to read created config: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, `"stack": "go-agent"`) {
		t.Errorf("expected config to contain '\"stack\": \"go-agent\"', got:\n%s", content)
	}
	if !strings.Contains(content, `"reviewed_stack"`) {
		t.Errorf("expected config to contain 'reviewed_stack', got:\n%s", content)
	}

	planOut, planErr, err := stackTestCommand(t, dir, "plan", "--no-ssh", "--no-dbus")
	if err != nil {
		t.Fatalf("bws plan failed in workspace with stack: %v\n%s\n%s", err, planOut, planErr)
	}
	if !strings.Contains(planOut, "go-agent") && !strings.Contains(planOut, "go-dev") {
		t.Errorf("expected plan output to reference stack components, got:\n%s", planOut)
	}
}

func TestStackSaveGenesisInvariant(t *testing.T) {
	stackTestHome(t)
	emptyDir := t.TempDir()

	out, errOut, err := stackTestCommand(t, emptyDir, "stack", "save", "genesis-test", "--no-verify")
	if err == nil {
		t.Fatalf("expected bws stack save to fail in non-workspace dir, got success: %s %s", out, errOut)
	}
	if !strings.Contains(errOut, "genesis invariant") && !strings.Contains(out, "genesis invariant") {
		t.Errorf("expected error message mentioning genesis invariant, got:\n%s\n%s", errOut, out)
	}

	wsDir := t.TempDir()
	initOut, initErr, err := stackTestCommand(t, wsDir, "init", "-p", "editor,git", "-y")
	if err != nil {
		t.Fatalf("bws init failed: %v\n%s\n%s", err, initOut, initErr)
	}

	saveOut, saveErr, err := stackTestCommand(t, wsDir, "stack", "save", "my-saved-stack", "--no-verify", "-t", "My Custom Stack")
	if err != nil {
		t.Fatalf("bws stack save in valid workspace failed: %v\n%s\n%s", err, saveOut, saveErr)
	}
	if !strings.Contains(saveOut, "my-saved-stack") {
		t.Errorf("expected save output to mention stack name, got:\n%s", saveOut)
	}

	listOut, _, err := stackTestCommand(t, wsDir, "stack", "list")
	if err != nil {
		t.Fatalf("bws stack list failed: %v", err)
	}
	if !strings.Contains(listOut, "User Saved Stacks:") || !strings.Contains(listOut, "my-saved-stack") {
		t.Errorf("expected user stack list to show my-saved-stack, got:\n%s", listOut)
	}

	showOut, _, err := stackTestCommand(t, wsDir, "stack", "show", "my-saved-stack")
	if err != nil {
		t.Fatalf("bws stack show my-saved-stack failed: %v", err)
	}
	if !strings.Contains(showOut, "Provenance:") || !strings.Contains(showOut, "Workspace:") {
		t.Errorf("expected show to include provenance, got:\n%s", showOut)
	}
}

func TestStackUpdateWorkflow(t *testing.T) {
	stackTestHome(t)
	wsDir := t.TempDir()

	_, _, err := stackTestCommand(t, wsDir, "init", "-p", "editor", "-y")
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}
	_, _, err = stackTestCommand(t, wsDir, "stack", "save", "updatable-stack", "--no-verify")
	if err != nil {
		t.Fatalf("stack save failed: %v", err)
	}

	consumerDir := t.TempDir()
	_, _, err = stackTestCommand(t, consumerDir, "init", "--stack", "updatable-stack", "-y")
	if err != nil {
		t.Fatalf("init consumer failed: %v", err)
	}

	upOut, _, err := stackTestCommand(t, consumerDir, "stack", "update")
	if err != nil {
		t.Fatalf("bws stack update failed: %v", err)
	}
	if !strings.Contains(upOut, "already up to date") {
		t.Errorf("expected 'already up to date', got:\n%s", upOut)
	}

	userStackPath := filepath.Join(os.Getenv("HOME"), ".config", "bws", "stacks", "updatable-stack.json")
	data, err := os.ReadFile(userStackPath)
	if err != nil {
		t.Fatalf("failed to read user stack: %v", err)
	}
	var s stack.Stack
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("unmarshal user stack: %v", err)
	}
	s.Profiles = append(s.Profiles, "git")
	updatedData, _ := stack.ToJSON(&s)
	if err := os.WriteFile(userStackPath, updatedData, 0644); err != nil {
		t.Fatalf("failed to update user stack file: %v", err)
	}

	dryOut, dryErr, err := stackTestCommand(t, consumerDir, "stack", "update", "-n")
	if err != nil {
		t.Fatalf("dry run failed: %v\n%s\n%s", err, dryOut, dryErr)
	}
	if !strings.Contains(dryOut, "[dry-run]") {
		t.Errorf("expected dry run message, got:\n%s", dryOut)
	}

	applyOut, applyErr, err := stackTestCommand(t, consumerDir, "stack", "update", "-y")
	if err != nil {
		t.Fatalf("apply update failed: %v\n%s\n%s", err, applyOut, applyErr)
	}
	if !strings.Contains(applyOut, "Successfully updated stack") {
		t.Errorf("expected success message, got:\n%s", applyOut)
	}

	bakPath := filepath.Join(consumerDir, ".bws", "config.jsonc.bak")
	if _, err := os.Stat(bakPath); err != nil {
		t.Errorf("expected backup file %s to exist", bakPath)
	}

	finalUpOut, _, err := stackTestCommand(t, consumerDir, "stack", "update")
	if err != nil {
		t.Fatalf("post-update check failed: %v", err)
	}
	if !strings.Contains(finalUpOut, "already up to date") {
		t.Errorf("expected 'already up to date' after upgrade, got:\n%s", finalUpOut)
	}
}

func TestInitStackRankingWithMarkers(t *testing.T) {
	stackTestHome(t)

	goDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(goDir, "go.mod"), []byte("module example.com/test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	out, errOut, _ := stackTestCommand(t, goDir, "init", "-n")
	combined := out + "\n" + errOut
	if !strings.Contains(combined, "go-agent") || !strings.Contains(combined, "go.mod") {
		t.Errorf("expected ranking to suggest go-agent for go.mod, got:\n%s", combined)
	}

	pyDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(pyDir, "pyproject.toml"), []byte("[project]\nname = \"test\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	out, errOut, _ = stackTestCommand(t, pyDir, "init", "-n")
	combined = out + "\n" + errOut
	if !strings.Contains(combined, "python-uv") || !strings.Contains(combined, "pyproject.toml") {
		t.Errorf("expected ranking to suggest python-uv for pyproject.toml, got:\n%s", combined)
	}

	rustDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rustDir, "Cargo.toml"), []byte("[package]\nname = \"test\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	out, errOut, _ = stackTestCommand(t, rustDir, "init", "-n")
	combined = out + "\n" + errOut
	if !strings.Contains(combined, "rust-dev") || !strings.Contains(combined, "Cargo.toml") {
		t.Errorf("expected ranking to suggest rust-dev for Cargo.toml, got:\n%s", combined)
	}
}
