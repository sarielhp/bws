package gitworkflow

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPruneMergedBranchesAndTempDirs(t *testing.T) {
	repoDir := setupTestRepo(t)
	baseTemp := t.TempDir()

	// Create fake agent temp directories
	agentDir1 := filepath.Join(baseTemp, "agent_12345")
	agentDir2 := filepath.Join(baseTemp, "agent_67890")
	otherDir := filepath.Join(baseTemp, "not_agent_dir")
	_ = os.MkdirAll(agentDir1, 0755)
	_ = os.MkdirAll(agentDir2, 0755)
	_ = os.MkdirAll(otherDir, 0755)

	// Create a merged branch: bws-agent-merged-a
	_ = runCmd(repoDir, "git", "checkout", "-b", "bws-agent-merged-a")
	_ = os.WriteFile(filepath.Join(repoDir, "fileA.txt"), []byte("data A"), 0644)
	_ = runCmd(repoDir, "git", "add", "fileA.txt")
	_ = runCmd(repoDir, "git", "commit", "-m", "commit A")

	// Merge into master/main
	_ = runCmd(repoDir, "git", "checkout", "master")
	_ = runCmd(repoDir, "git", "checkout", "main")
	_ = runCmd(repoDir, "git", "merge", "bws-agent-merged-a")

	// Create an unmerged branch: bws-agent-unmerged-b
	_ = runCmd(repoDir, "git", "checkout", "-b", "bws-agent-unmerged-b")
	_ = os.WriteFile(filepath.Join(repoDir, "fileB.txt"), []byte("data B"), 0644)
	_ = runCmd(repoDir, "git", "add", "fileB.txt")
	_ = runCmd(repoDir, "git", "commit", "-m", "commit B")

	// Switch back to master/main
	_ = runCmd(repoDir, "git", "checkout", "master")
	_ = runCmd(repoDir, "git", "checkout", "main")

	// 1. Dry run test
	var dryBuf bytes.Buffer
	err := PruneInDir(repoDir, PruneOptions{
		DryRun:  true,
		TempDir: baseTemp,
		Writer:  &dryBuf,
	})
	if err != nil {
		t.Fatalf("dry-run PruneInDir failed: %v", err)
	}
	dryOut := dryBuf.String()
	if !strings.Contains(dryOut, "[dry-run]") {
		t.Errorf("expected '[dry-run]' in dry-run output, got:\n%s", dryOut)
	}
	if !strings.Contains(dryOut, "bws-agent-merged-a") {
		t.Errorf("expected 'bws-agent-merged-a' in dry-run output, got:\n%s", dryOut)
	}
	if !strings.Contains(dryOut, "agent_12345") || !strings.Contains(dryOut, "agent_67890") {
		t.Errorf("expected temp dirs in dry-run output, got:\n%s", dryOut)
	}

	// Verify nothing was deleted during dry-run
	if _, err := os.Stat(agentDir1); os.IsNotExist(err) {
		t.Errorf("expected agentDir1 to exist after dry-run")
	}

	// 2. Real prune (default: only merged)
	var pruneBuf bytes.Buffer
	err = PruneInDir(repoDir, PruneOptions{
		All:     false,
		DryRun:  false,
		TempDir: baseTemp,
		Writer:  &pruneBuf,
	})
	if err != nil {
		t.Fatalf("PruneInDir failed: %v", err)
	}
	pruneOut := pruneBuf.String()
	if !strings.Contains(pruneOut, "Pruned 1 branch") || !strings.Contains(pruneOut, "bws-agent-merged-a") {
		t.Errorf("expected merged branch pruned in output, got:\n%s", pruneOut)
	}
	if !strings.Contains(pruneOut, "Skipped 1 unmerged branch") || !strings.Contains(pruneOut, "bws-agent-unmerged-b") {
		t.Errorf("expected unmerged branch skipped in output, got:\n%s", pruneOut)
	}

	// Verify temp dirs are removed but not_agent_dir remains
	if _, err := os.Stat(agentDir1); !os.IsNotExist(err) {
		t.Errorf("expected agentDir1 to be removed")
	}
	if _, err := os.Stat(agentDir2); !os.IsNotExist(err) {
		t.Errorf("expected agentDir2 to be removed")
	}
	if _, err := os.Stat(otherDir); os.IsNotExist(err) {
		t.Errorf("expected otherDir to be preserved")
	}

	// 3. Prune all (including unmerged)
	var pruneAllBuf bytes.Buffer
	err = PruneInDir(repoDir, PruneOptions{
		All:     true,
		DryRun:  false,
		TempDir: baseTemp,
		Writer:  &pruneAllBuf,
	})
	if err != nil {
		t.Fatalf("PruneInDir --all failed: %v", err)
	}
	pruneAllOut := pruneAllBuf.String()
	if !strings.Contains(pruneAllOut, "bws-agent-unmerged-b") {
		t.Errorf("expected unmerged branch pruned with --all, got:\n%s", pruneAllOut)
	}

	// 4. Nothing to prune test
	var emptyBuf bytes.Buffer
	err = PruneInDir(repoDir, PruneOptions{
		TempDir: baseTemp,
		Writer:  &emptyBuf,
	})
	if err != nil {
		t.Fatalf("PruneInDir on empty state failed: %v", err)
	}
	if !strings.Contains(emptyBuf.String(), "No bws-agent branches or temp directories to prune") {
		t.Errorf("expected 'No bws-agent branches or temp directories to prune', got:\n%s", emptyBuf.String())
	}
}

func TestPruneCurrentBranchSkipped(t *testing.T) {
	repoDir := setupTestRepo(t)
	baseTemp := t.TempDir()

	_ = runCmd(repoDir, "git", "checkout", "-b", "bws-agent-current")
	_ = os.WriteFile(filepath.Join(repoDir, "fileC.txt"), []byte("data C"), 0644)
	_ = runCmd(repoDir, "git", "add", "fileC.txt")
	_ = runCmd(repoDir, "git", "commit", "-m", "commit C")

	var buf bytes.Buffer
	err := PruneInDir(repoDir, PruneOptions{
		All:     true,
		TempDir: baseTemp,
		Writer:  &buf,
	})
	if err != nil {
		t.Fatalf("PruneInDir failed: %v", err)
	}

	// Current branch should still exist
	curr, _ := getCurrentBranch(repoDir)
	if curr != "bws-agent-current" {
		t.Errorf("expected current branch to remain 'bws-agent-current', got %q", curr)
	}
}
