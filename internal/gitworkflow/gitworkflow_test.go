package gitworkflow

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitWorkflowHelpers(t *testing.T) {
	tmpDir := t.TempDir()

	if err := runCmd(tmpDir, "git", "init"); err != nil {
		t.Fatal(err)
	}
	_ = runCmd(tmpDir, "git", "config", "user.email", "test@example.com")
	_ = runCmd(tmpDir, "git", "config", "user.name", "Test User")

	root, err := getGitRoot()
	if err != nil || root == "" {
		t.Fatalf("getGitRoot failed on host: %v", err)
	}

	testFile := filepath.Join(tmpDir, "hello.txt")
	_ = os.WriteFile(testFile, []byte("world"), 0644)

	dirty, err := checkDirty(tmpDir)
	if err != nil {
		t.Fatalf("checkDirty failed: %v", err)
	}
	if !dirty {
		t.Errorf("expected dirty=true, got false")
	}

	_ = runCmd(tmpDir, "git", "add", "hello.txt")
	_ = runCmd(tmpDir, "git", "commit", "-m", "initial")

	dirty, err = checkDirty(tmpDir)
	if err != nil || dirty {
		t.Errorf("expected clean repo, got dirty=%v, err=%v", dirty, err)
	}

	branch, err := getCurrentBranch(tmpDir)
	if err != nil || branch == "" {
		t.Errorf("getCurrentBranch failed: %v, got %q", err, branch)
	}
}

func TestBuildBwsArgs(t *testing.T) {
	opts := Options{
		Verbose: true,
		NoNet:   true,
		Proxy:   true,
		DBus:    true,
		NoInit:  true,
		Command: []string{"echo", "hi"},
	}

	args := buildBwsArgs(opts)
	argStr := strings.Join(args, " ")

	expectedTokens := []string{"run", "--no-ssh", "-v", "-N", "--proxy", "--dbus", "--no-init", "--", "echo", "hi"}
	for _, tok := range expectedTokens {
		if !strings.Contains(argStr, tok) {
			t.Errorf("expected argument %q in buildBwsArgs output, got: %s", tok, argStr)
		}
	}
}

func TestPromptTriageEOF(t *testing.T) {
	// Must return cleanly on EOF without hanging or looping
	promptTriage(strings.NewReader(""), "/tmp", "base-sha", "main", "bws-agent-test")
}

func TestPromptTriageInvalidAttempts(t *testing.T) {
	// 3 invalid attempts should exit cleanly
	promptTriage(strings.NewReader("invalid\nbad\nwrong\n"), "/tmp", "base-sha", "main", "bws-agent-test")
}

func TestSensitiveFilesExcluded(t *testing.T) {
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git", "info")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}

	excludeSensitiveFiles(tmpDir)

	content, err := os.ReadFile(filepath.Join(gitDir, "exclude"))
	if err != nil {
		t.Fatalf("reading exclude file failed: %v", err)
	}

	if !strings.Contains(string(content), ".bws/") || !strings.Contains(string(content), ".env*") {
		t.Errorf("expected .bws/ and .env* in exclude file, got: %s", string(content))
	}
}

func TestCheckoutAgentBranchExistingAndNew(t *testing.T) {
	hostRepo := t.TempDir()
	_ = runCmd(hostRepo, "git", "init")
	_ = runCmd(hostRepo, "git", "config", "user.email", "test@example.com")
	_ = runCmd(hostRepo, "git", "config", "user.name", "Test User")
	_ = os.WriteFile(filepath.Join(hostRepo, "README.md"), []byte("init"), 0644)
	_ = runCmd(hostRepo, "git", "add", "README.md")
	_ = runCmd(hostRepo, "git", "commit", "-m", "init")

	// Create an existing branch on host
	_ = runCmd(hostRepo, "git", "branch", "bws-agent-existing")

	// Clone to tempDir
	cloneDir := t.TempDir()
	_ = runCmd(hostRepo, "git", "clone", "--shared", fmt.Sprintf("file://%s", hostRepo), cloneDir)

	// 1. Checkout existing branch
	if err := checkoutAgentBranch(hostRepo, cloneDir, "bws-agent-existing"); err != nil {
		t.Fatalf("checkoutAgentBranch existing branch failed: %v", err)
	}

	// 2. Checkout new branch
	if err := checkoutAgentBranch(hostRepo, cloneDir, "bws-agent-new"); err != nil {
		t.Fatalf("checkoutAgentBranch new branch failed: %v", err)
	}
}

func TestSquashKeepsBranchWhenCommitFails(t *testing.T) {
	repo := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		if err := runCmd(repo, "git", args...); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}
	git("init", "-q", "-b", "main")
	git("config", "user.email", "test@example.com")
	git("config", "user.name", "Test User")
	git("commit", "-q", "--allow-empty", "-m", "base")
	git("checkout", "-q", "-b", "bws-agent-x")
	if err := os.WriteFile(filepath.Join(repo, "f.txt"), []byte("work"), 0644); err != nil {
		t.Fatal(err)
	}
	git("add", "f.txt")
	git("commit", "-q", "-m", "agent work")
	git("checkout", "-q", "main")

	hook := filepath.Join(repo, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nexit 1\n"), 0755); err != nil {
		t.Fatal(err)
	}

	promptTriage(strings.NewReader("s\n"), repo, "HEAD", "main", "bws-agent-x")

	if err := runCmd(repo, "git", "rev-parse", "--verify", "refs/heads/bws-agent-x"); err != nil {
		t.Fatalf("agent branch deleted after failed squash commit")
	}
}

func TestSandboxResultWrapsExitError(t *testing.T) {
	if sandboxResult(nil) != nil {
		t.Fatal("nil error must stay nil")
	}
	err := exec.Command("sh", "-c", "exit 3").Run()
	var exitErr *exec.ExitError
	if !errors.As(sandboxResult(err), &exitErr) || exitErr.ExitCode() != 3 {
		t.Fatalf("exit code not preserved: %v", sandboxResult(err))
	}
}
