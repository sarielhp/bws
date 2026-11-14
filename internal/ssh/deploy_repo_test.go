package ssh

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGitHubRemote(t *testing.T) {
	cases := map[string]string{
		"git@github.com:owner/repo.git":          "owner/repo",
		"https://github.com/owner/repo":          "owner/repo",
		"https://github.com/owner/my.repo.git":   "owner/my.repo",
		"ssh://git@github.com/owner/repo.git/":   "owner/repo",
		"git@gitlab.com:owner/repo.git":          "",
		"https://github.com/owner/repo/extra":    "",
		"https://github.com/../repo":             "",
		"https://github.com/owner/repo;rm -rf ~": "",
	}
	for in, want := range cases {
		if got := parseGitHubRemote(in); got != want {
			t.Errorf("parseGitHubRemote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolveDeployRepoPinned(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if got, err := ResolveDeployRepo("owner/repo", t.TempDir()); err != nil || got != "owner/repo" {
		t.Fatalf("pinned = %q, %v", got, err)
	}
	for _, bad := range []string{"owner", "../x", "a/b/c", "a/.."} {
		if _, err := ResolveDeployRepo(bad, t.TempDir()); err == nil {
			t.Errorf("invalid pin %q accepted", bad)
		}
	}
}

// TestResolveDeployRepoRefusesRewrittenOrigin covers a sandbox rewriting
// remote.origin.url to obtain a deploy key for another repository.
func TestResolveDeployRepoRefusesRewrittenOrigin(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	t.Setenv("HOME", t.TempDir())
	ws := t.TempDir()
	run := func(args ...string) {
		if out, err := exec.Command("git", append([]string{"-C", ws}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run("init", "-q")
	run("remote", "add", "origin", "git@github.com:owner/project.git")

	if got, err := ResolveDeployRepo("", ws); err != nil || got != "owner/project" {
		t.Fatalf("first use = %q, %v", got, err)
	}
	if got, err := ResolveDeployRepo("", ws); err != nil || got != "owner/project" {
		t.Fatalf("second use = %q, %v", got, err)
	}

	run("remote", "set-url", "origin", "git@github.com:owner/other.git")
	got, err := ResolveDeployRepo("", ws)
	if err == nil || got != "" {
		t.Fatalf("rewritten origin accepted: %q, %v", got, err)
	}
	if !strings.Contains(err.Error(), "deploy_key_repo") {
		t.Errorf("error does not explain how to approve: %v", err)
	}
	if got, err := ResolveDeployRepo("owner/other", ws); err != nil || got != "owner/other" {
		t.Fatalf("pin did not override record: %q, %v", got, err)
	}
}

func TestStartSessionAgentWithoutKeysForwardsNothing(t *testing.T) {
	if got := StartSessionAgent(nil); got != "" {
		StopSessionAgents()
		t.Fatalf("agent started without keys: %s", got)
	}
}

// TestSessionAgentsAreIsolated checks that each session has its own agent
// holding only its key, and that stopping removes the socket.
func TestSessionAgentsAreIsolated(t *testing.T) {
	for _, bin := range []string{"ssh-agent", "ssh-add", "ssh-keygen"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skip(bin + " not installed")
		}
	}
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	keys := make([]string, 2)
	for i := range keys {
		keys[i] = filepath.Join(dir, "key"+string(rune('a'+i)))
		if out, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", keys[i]).CombinedOutput(); err != nil {
			t.Fatalf("ssh-keygen: %v: %s", err, out)
		}
	}
	a := StartSessionAgent(keys[:1])
	b := StartSessionAgent(keys[1:])
	defer StopSessionAgents()
	if a == "" || b == "" || a == b {
		t.Fatalf("expected two distinct agents, got %q and %q", a, b)
	}
	for sock, key := range map[string]string{a: keys[0], b: keys[1]} {
		cmd := exec.Command("ssh-add", "-L")
		cmd.Env = append(os.Environ(), "SSH_AUTH_SOCK="+sock)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("ssh-add -L: %v", err)
		}
		pub, _ := os.ReadFile(key + ".pub")
		fields := strings.Fields(string(pub))
		if strings.Count(strings.TrimSpace(string(out)), "\n") != 0 || !strings.Contains(string(out), fields[1]) {
			t.Errorf("agent %s holds %q, want only %s", sock, out, key)
		}
	}
	StopSessionAgents()
	for _, sock := range []string{a, b} {
		if _, err := os.Stat(sock); !os.IsNotExist(err) {
			t.Errorf("socket %s survived StopSessionAgents", sock)
		}
	}
}
