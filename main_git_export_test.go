package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"bws/internal/config"
)

func fixtureGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return string(output)
}

func TestGitExportConfinesUntrustedMetadata(t *testing.T) {
	repo := t.TempDir()
	fixtureGit(t, repo, "init", "-b", "agent")
	fixtureGit(t, repo, "config", "user.name", "Test")
	fixtureGit(t, repo, "config", "user.email", "test@example.invalid")
	if err := os.WriteFile(filepath.Join(repo, "initial"), []byte("base"), 0644); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, repo, "add", ".")
	fixtureGit(t, repo, "commit", "-m", "base")
	marker := filepath.Join(t.TempDir(), "outside-sandbox")
	script := "#!/bin/sh\nprintf escaped > '" + marker + "'\ncat\n"
	for _, name := range []string{"post-commit", "monitor", "filter"} {
		if err := os.WriteFile(filepath.Join(repo, ".git", "hooks", name), []byte(script), 0755); err != nil {
			t.Fatal(err)
		}
	}
	fixtureGit(t, repo, "config", "core.fsmonitor", ".git/hooks/monitor")
	fixtureGit(t, repo, "config", "filter.attack.clean", ".git/hooks/filter")
	if err := os.WriteFile(filepath.Join(repo, ".gitattributes"), []byte("change filter=attack\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "change"), []byte("preserved change\n"), 0644); err != nil {
		t.Fatal(err)
	}
	var bundle bytes.Buffer
	if err := exportAgentBundle(repo, "agent", &bundle); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("host marker was accessed: %v", err)
	}
	path := filepath.Join(t.TempDir(), "agent.bundle")
	if err := os.WriteFile(path, bundle.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	receiver := t.TempDir()
	fixtureGit(t, receiver, "init")
	fixtureGit(t, receiver, "fetch", path, "refs/heads/agent:refs/heads/agent")
	if got := fixtureGit(t, receiver, "show", "agent:change"); got != "preserved change\n" {
		t.Fatalf("lost changes: %q", got)
	}
}

func TestGitExportRejectsUncommittableChanges(t *testing.T) {
	repo := t.TempDir()
	fixtureGit(t, repo, "init", "-b", "agent")
	if err := os.WriteFile(filepath.Join(repo, "change"), []byte("keep me"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "index.lock"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	var bundle bytes.Buffer
	if err := exportAgentBundle(repo, "agent", &bundle); err == nil {
		t.Fatal("failed commit reported as successful export")
	}
	if bundle.Len() != 0 {
		t.Fatal("partial result exported despite failed commit")
	}
	if _, err := os.Stat(filepath.Join(repo, "change")); err != nil {
		t.Fatal("workspace lost", err)
	}
}

func TestGitWorkflowImportsIsolatedBundle(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repo := t.TempDir()
	fixtureGit(t, repo, "init", "-b", "main")
	fixtureGit(t, repo, "config", "user.name", "Test")
	fixtureGit(t, repo, "config", "user.email", "test@example.invalid")
	fixtureGit(t, repo, "commit", "--allow-empty", "-m", "base")
	if err := os.Mkdir(filepath.Join(repo, ".bws"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := config.WriteTrustedFile(filepath.Join(repo, ".bws", "config.jsonc"), []byte("{}")); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bwPath, "gw", "--allow-dirty", "-b", "agent-export", "--", "sh", "-c", "printf 'complete workflow\\n' > change")
	cmd.Dir = repo
	cmd.Stdin = strings.NewReader("k\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("workflow: %v\n%s", err, out)
	}
	if got := fixtureGit(t, repo, "show", "agent-export:change"); got != "complete workflow\n" {
		t.Fatalf("missing imported change: %q", got)
	}
	if _, err := os.Stat(filepath.Join(repo, "change")); !os.IsNotExist(err) {
		t.Fatal("host working tree changed before merge")
	}
}
