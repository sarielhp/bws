package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sarielhp/clihelp/clihelptest"
)

func TestGwListAndPruneCLI(t *testing.T) {
	ensureGlobalConfig(t)
	app := buildApp()

	// 1. Test help on gw
	res := clihelptest.Execute(app, []string{"gw", "--help"})
	res.AssertNoError(t)
	res.AssertStdoutContains(t, "list")
	res.AssertStdoutContains(t, "prune")

	// 2. Test help on gw list
	res = clihelptest.Execute(app, []string{"gw", "list", "--help"})
	res.AssertNoError(t)
	res.AssertStdoutContains(t, "--merged")
	res.AssertStdoutContains(t, "--unmerged")

	// 3. Test help on gw prune
	res = clihelptest.Execute(app, []string{"gw", "prune", "--help"})
	res.AssertNoError(t)
	res.AssertStdoutContains(t, "--all")
	res.AssertStdoutContains(t, "--dry-run")

	// 4. Test execution of gw list
	res = clihelptest.Execute(app, []string{"gw", "list"})
	res.AssertNoError(t)

	// 5. Test execution of gw prune dry run
	res = clihelptest.Execute(app, []string{"gw", "prune", "-n"})
	res.AssertNoError(t)
}

// gwFixtureInits a git repository with one merged agent branch and one
// unmerged agent branch, returning its path. It skips when git is unavailable.
func gwFixtureInits(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	git := func(args ...string) {
		_ = exec.Command("git", append([]string{"-C", tmpDir}, args...)...).Run()
	}
	if err := exec.Command("git", "init", tmpDir).Run(); err != nil {
		t.Skip("git init failed, skipping integration test")
	}
	git("config", "user.email", "agent@example.com")
	git("config", "user.name", "Agent")
	git("config", "commit.gpgsign", "false")

	_ = os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("test"), 0644)
	git("add", "README.md")
	git("commit", "-m", "init")

	// A branch that will be merged into main.
	git("checkout", "-b", "bws-agent-branch1")
	_ = os.WriteFile(filepath.Join(tmpDir, "f1.txt"), []byte("f1"), 0644)
	git("add", "f1.txt")
	git("commit", "-m", "feat1")
	git("checkout", "master")
	git("checkout", "main")
	git("merge", "bws-agent-branch1")

	// A branch left unmerged.
	git("checkout", "-b", "bws-agent-branch2")
	_ = os.WriteFile(filepath.Join(tmpDir, "f2.txt"), []byte("f2"), 0644)
	git("add", "f2.txt")
	git("commit", "-m", "feat2")
	git("checkout", "master")
	git("checkout", "main")
	return tmpDir
}

// gwRun runs the bws binary in dir and returns combined output, failing the
// test if the command errors.
func gwRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bwPath, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
	}
	return string(out)
}

func TestGwIntegrationInRepo(t *testing.T) {
	tmpDir := gwFixtureInits(t)
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("bws binary not built, skipping")
	}

	out := gwRun(t, tmpDir, "gw", "list")
	if !strings.Contains(out, "bws-agent-branch1") || !strings.Contains(out, "bws-agent-branch2") {
		t.Errorf("expected branch1 and branch2 in list output, got:\n%s", out)
	}

	if out := gwRun(t, tmpDir, "gw", "prune", "-n"); !strings.Contains(out, "bws-agent-branch1") {
		t.Errorf("expected branch1 in prune dry run, got:\n%s", out)
	}

	if out := gwRun(t, tmpDir, "gw", "prune"); !strings.Contains(out, "bws-agent-branch1") {
		t.Errorf("expected branch1 pruned, got:\n%s", out)
	}

	branchesOut, _ := exec.Command("git", "-C", tmpDir, "branch").CombinedOutput()
	if strings.Contains(string(branchesOut), "bws-agent-branch1") {
		t.Errorf("bws-agent-branch1 should have been deleted")
	}
	if !strings.Contains(string(branchesOut), "bws-agent-branch2") {
		t.Errorf("bws-agent-branch2 should be preserved")
	}

	if out := gwRun(t, tmpDir, "gw", "prune", "-a"); !strings.Contains(out, "bws-agent-branch2") {
		t.Errorf("expected branch2 pruned with -a, got:\n%s", out)
	}
}
