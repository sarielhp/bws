package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sarielhp/clihelp"
)

func TestGwListAndPruneCLI(t *testing.T) {
	ensureGlobalConfig(t)
	app := buildApp()

	// 1. Test help on gw
	res := clihelp.TestExecute(app, []string{"gw", "--help"})
	res.AssertNoError(t)
	res.AssertStdoutContains(t, "list")
	res.AssertStdoutContains(t, "prune")

	// 2. Test help on gw list
	res = clihelp.TestExecute(app, []string{"gw", "list", "--help"})
	res.AssertNoError(t)
	res.AssertStdoutContains(t, "--merged")
	res.AssertStdoutContains(t, "--unmerged")

	// 3. Test help on gw prune
	res = clihelp.TestExecute(app, []string{"gw", "prune", "--help"})
	res.AssertNoError(t)
	res.AssertStdoutContains(t, "--all")
	res.AssertStdoutContains(t, "--dry-run")

	// 4. Test execution of gw list
	res = clihelp.TestExecute(app, []string{"gw", "list"})
	res.AssertNoError(t)

	// 5. Test execution of gw prune dry run
	res = clihelp.TestExecute(app, []string{"gw", "prune", "-n"})
	res.AssertNoError(t)
}

func TestGwIntegrationInRepo(t *testing.T) {
	tmpDir := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Skip("git init failed, skipping integration test")
	}
	_ = exec.Command("git", "-C", tmpDir, "config", "user.email", "agent@example.com").Run()
	_ = exec.Command("git", "-C", tmpDir, "config", "user.name", "Agent").Run()
	_ = exec.Command("git", "-C", tmpDir, "config", "commit.gpgsign", "false").Run()

	_ = os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("test"), 0644)
	_ = exec.Command("git", "-C", tmpDir, "add", "README.md").Run()
	_ = exec.Command("git", "-C", tmpDir, "commit", "-m", "init").Run()

	// Create merged and unmerged agent branches
	_ = exec.Command("git", "-C", tmpDir, "checkout", "-b", "bws-agent-branch1").Run()
	_ = os.WriteFile(filepath.Join(tmpDir, "f1.txt"), []byte("f1"), 0644)
	_ = exec.Command("git", "-C", tmpDir, "add", "f1.txt").Run()
	_ = exec.Command("git", "-C", tmpDir, "commit", "-m", "feat1").Run()

	_ = exec.Command("git", "-C", tmpDir, "checkout", "master").Run()
	_ = exec.Command("git", "-C", tmpDir, "checkout", "main").Run()
	_ = exec.Command("git", "-C", tmpDir, "merge", "bws-agent-branch1").Run()

	_ = exec.Command("git", "-C", tmpDir, "checkout", "-b", "bws-agent-branch2").Run()
	_ = os.WriteFile(filepath.Join(tmpDir, "f2.txt"), []byte("f2"), 0644)
	_ = exec.Command("git", "-C", tmpDir, "add", "f2.txt").Run()
	_ = exec.Command("git", "-C", tmpDir, "commit", "-m", "feat2").Run()

	_ = exec.Command("git", "-C", tmpDir, "checkout", "master").Run()
	_ = exec.Command("git", "-C", tmpDir, "checkout", "main").Run()

	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("bws binary not built, skipping")
	}

	// Test bws gw list
	listCmd := exec.Command(bwPath, "gw", "list")
	listCmd.Dir = tmpDir
	out, err := listCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws gw list failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "bws-agent-branch1") || !strings.Contains(string(out), "bws-agent-branch2") {
		t.Errorf("expected branch1 and branch2 in list output, got:\n%s", string(out))
	}

	// Test bws gw prune -n (dry run)
	pruneDryCmd := exec.Command(bwPath, "gw", "prune", "-n")
	pruneDryCmd.Dir = tmpDir
	out, err = pruneDryCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws gw prune -n failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "bws-agent-branch1") {
		t.Errorf("expected branch1 in prune dry run, got:\n%s", string(out))
	}

	// Test bws gw prune (real run)
	pruneCmd := exec.Command(bwPath, "gw", "prune")
	pruneCmd.Dir = tmpDir
	out, err = pruneCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws gw prune failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "bws-agent-branch1") {
		t.Errorf("expected branch1 pruned, got:\n%s", string(out))
	}

	// Verify branch1 was deleted and branch2 remained
	branchesOut, _ := exec.Command("git", "-C", tmpDir, "branch").CombinedOutput()
	if strings.Contains(string(branchesOut), "bws-agent-branch1") {
		t.Errorf("bws-agent-branch1 should have been deleted")
	}
	if !strings.Contains(string(branchesOut), "bws-agent-branch2") {
		t.Errorf("bws-agent-branch2 should be preserved")
	}

	// Test bws gw prune -a (prune all including unmerged)
	pruneAllCmd := exec.Command(bwPath, "gw", "prune", "-a")
	pruneAllCmd.Dir = tmpDir
	out, err = pruneAllCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws gw prune -a failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "bws-agent-branch2") {
		t.Errorf("expected branch2 pruned with -a, got:\n%s", string(out))
	}
}
