package bwrap

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"bws/internal/config"
)

// gitWorkspace creates HOME/project as a Git repository and returns it.
func gitWorkspace(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	ws := filepath.Join(home, "project")
	if err := os.MkdirAll(ws, 0755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", "-q", ws).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	return ws
}

func gitSet(t *testing.T, ws, key, val string) {
	t.Helper()
	if out, err := exec.Command("git", "-C", ws, "config", key, val).CombinedOutput(); err != nil {
		t.Fatalf("git config: %v: %s", err, out)
	}
}

// hasMount reports whether args contain "flag src dst".
func hasMount(args []string, flag, src, dst string) bool {
	for i := 0; i+2 < len(args); i++ {
		if args[i] == flag && args[i+1] == src && args[i+2] == dst {
			return true
		}
	}
	return false
}

func TestGitProtectMountsConfigAndHooks(t *testing.T) {
	ws := gitWorkspace(t)
	gitDir := filepath.Join(ws, ".git")
	if err := os.RemoveAll(filepath.Join(gitDir, "hooks")); err != nil {
		t.Fatal(err)
	}
	var args []string
	addGitProtectArgs(&args, &config.Config{}, ws, false, false)

	cfgFile := filepath.Join(gitDir, "config")
	hooks := filepath.Join(gitDir, "hooks")
	for _, m := range [][3]string{
		{"--bind", gitDir, gitDir},
		{"--ro-bind", cfgFile, cfgFile},
		{"--ro-bind", hooks, hooks},
	} {
		if !hasMount(args, m[0], m[1], m[2]) {
			t.Errorf("missing %v in %v", m, args)
		}
	}
	if !dirExists(hooks) {
		t.Error("missing hooks directory was not created before being protected")
	}
}

func TestGitProtectFollowsHooksPathAndIncludes(t *testing.T) {
	ws := gitWorkspace(t)
	gitSet(t, ws, "core.hooksPath", "tools/hooks")
	gitSet(t, ws, "include.path", "../shared/git.inc")
	var args []string
	addGitProtectArgs(&args, &config.Config{}, ws, false, false)

	hooks := filepath.Join(ws, "tools", "hooks")
	if !hasMount(args, "--bind", filepath.Join(ws, "tools"), filepath.Join(ws, "tools")) {
		t.Errorf("ancestor of core.hooksPath not pinned: %v", args)
	}
	if !hasMount(args, "--ro-bind", hooks, hooks) {
		t.Errorf("core.hooksPath not read-only: %v", args)
	}
	inc := filepath.Join(ws, "shared", "git.inc")
	if !hasMount(args, "--ro-bind", "/dev/null", inc) {
		t.Errorf("missing include target not blocked: %v", args)
	}
}

func TestGitProtectSkipsGitDirOutsideWorkspace(t *testing.T) {
	ws := gitWorkspace(t)
	linked := filepath.Join(t.TempDir(), "linked")
	if err := os.MkdirAll(linked, 0755); err != nil {
		t.Fatal(err)
	}
	gitFile := "gitdir: " + filepath.Join(ws, ".git") + "\n"
	if err := os.WriteFile(filepath.Join(linked, ".git"), []byte(gitFile), 0644); err != nil {
		t.Fatal(err)
	}
	var args []string
	addGitProtectArgs(&args, &config.Config{}, linked, false, false)
	if strings.Contains(strings.Join(args, " "), ws) {
		t.Errorf("Git directory outside the workspace was mounted: %v", args)
	}
}

func TestGitProtectDisabled(t *testing.T) {
	ws := gitWorkspace(t)
	off := false
	var args []string
	addGitProtectArgs(&args, &config.Config{Features: &config.FeaturesConfig{ProtectGit: &off}}, ws, false, false)
	if len(args) != 0 {
		t.Errorf("protect_git=false still produced mounts: %v", args)
	}
}

// TestGitProtectBlocksEscapesInBwrap runs the attacks against a real sandbox.
func TestGitProtectBlocksEscapesInBwrap(t *testing.T) {
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bwrap not installed")
	}
	ws := gitWorkspace(t)
	gitSet(t, ws, "core.hooksPath", "tools/hooks")
	// An include inside .git/hooks must not leave hooks/ writable.
	gitSet(t, ws, "include.path", "hooks/evil.inc")
	args := []string{"--ro-bind", "/", "/", "--dev", "/dev", "--proc", "/proc", "--bind", ws, ws}
	addGitProtectArgs(&args, &config.Config{}, ws, false, false)
	script := `
fail() { echo "ESCAPE: $1"; exit 1; }
echo x >> .git/config 2>/dev/null && fail "wrote .git/config"
touch .git/hooks/pre-commit 2>/dev/null && fail "created hook"
mv .git .git-moved 2>/dev/null && fail "renamed .git"
mv tools tools-moved 2>/dev/null && fail "renamed hooksPath parent"
touch tools/hooks/pre-commit 2>/dev/null && fail "wrote core.hooksPath"
touch .git/probe || fail "git directory not writable"
echo ok`
	args = append(args, "--chdir", ws, "/bin/sh", "-c", script)
	out, err := exec.Command("bwrap", args...).CombinedOutput()
	if err != nil || !strings.Contains(string(out), "ok") {
		t.Fatalf("bwrap: %v\n%s", err, out)
	}
}
