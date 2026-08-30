package gitworkflow

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	if err := runCmd(dir, "git", "init"); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	_ = runCmd(dir, "git", "config", "user.email", "agent@example.com")
	_ = runCmd(dir, "git", "config", "user.name", "Agent Tester")
	_ = runCmd(dir, "git", "config", "commit.gpgsign", "false")

	// Create initial commit on main
	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# Test Repo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_ = runCmd(dir, "git", "add", "README.md")
	_ = runCmd(dir, "git", "commit", "-m", "initial commit")

	return dir
}

func TestListEmpty(t *testing.T) {
	repoDir := setupTestRepo(t)
	var buf bytes.Buffer

	err := ListInDir(repoDir, ListOptions{Writer: &buf})
	if err != nil {
		t.Fatalf("ListInDir failed: %v", err)
	}

	if !strings.Contains(buf.String(), "No bws-agent branches found") {
		t.Errorf("expected 'No bws-agent branches found', got: %s", buf.String())
	}
}

func TestListNotInGitRepo(t *testing.T) {
	nonRepo := t.TempDir()
	var buf bytes.Buffer

	err := ListInDir(nonRepo, ListOptions{Writer: &buf})
	if err == nil {
		t.Fatal("expected error when running outside git repo, got nil")
	}
}

func TestListBranchesAndFiltering(t *testing.T) {
	repoDir := setupTestRepo(t)

	// 1. Create a merged branch: bws-agent-merged-1
	_ = runCmd(repoDir, "git", "checkout", "-b", "bws-agent-merged-1")
	file1 := filepath.Join(repoDir, "feature1.txt")
	_ = os.WriteFile(file1, []byte("feature 1"), 0644)
	_ = runCmd(repoDir, "git", "add", "feature1.txt")
	_ = runCmd(repoDir, "git", "commit", "-m", "add feature 1")
	// Switch back to master/main and merge it
	mainBranch, _ := getCurrentBranch(repoDir)
	_ = runCmd(repoDir, "git", "checkout", "master")
	_ = runCmd(repoDir, "git", "checkout", "main")
	_ = runCmd(repoDir, "git", "merge", "bws-agent-merged-1")

	// 2. Create an unmerged branch: bws-agent-wip-2
	_ = runCmd(repoDir, "git", "checkout", "-b", "bws-agent-wip-2")
	file2 := filepath.Join(repoDir, "wip.txt")
	_ = os.WriteFile(file2, []byte("wip work"), 0644)
	_ = runCmd(repoDir, "git", "add", "wip.txt")
	_ = runCmd(repoDir, "git", "commit", "-m", "work in progress")

	// Switch back to main branch
	curr, _ := getCurrentBranch(repoDir)
	if curr == "bws-agent-wip-2" {
		_ = runCmd(repoDir, "git", "checkout", mainBranch)
	}

	// 3. Test GetAgentBranches
	branches, err := GetAgentBranches(repoDir)
	if err != nil {
		t.Fatalf("GetAgentBranches failed: %v", err)
	}
	if len(branches) != 2 {
		t.Fatalf("expected 2 agent branches, got %d", len(branches))
	}

	// 4. Test List all
	var bufAll bytes.Buffer
	err = ListInDir(repoDir, ListOptions{Writer: &bufAll})
	if err != nil {
		t.Fatalf("ListInDir failed: %v", err)
	}
	outAll := bufAll.String()
	if !strings.Contains(outAll, "bws-agent-merged-1") || !strings.Contains(outAll, "bws-agent-wip-2") {
		t.Errorf("expected both branches in list output, got:\n%s", outAll)
	}
	if !strings.Contains(outAll, "BRANCH") || !strings.Contains(outAll, "COMMIT") {
		t.Errorf("expected table header in list output, got:\n%s", outAll)
	}

	// 5. Test List --merged
	var bufMerged bytes.Buffer
	err = ListInDir(repoDir, ListOptions{MergedOnly: true, Writer: &bufMerged})
	if err != nil {
		t.Fatalf("ListInDir --merged failed: %v", err)
	}
	outMerged := bufMerged.String()
	if !strings.Contains(outMerged, "bws-agent-merged-1") {
		t.Errorf("expected bws-agent-merged-1 in --merged output, got:\n%s", outMerged)
	}
	if strings.Contains(outMerged, "bws-agent-wip-2") {
		t.Errorf("did not expect unmerged branch in --merged output, got:\n%s", outMerged)
	}

	// 6. Test List --unmerged
	var bufUnmerged bytes.Buffer
	err = ListInDir(repoDir, ListOptions{UnmergedOnly: true, Writer: &bufUnmerged})
	if err != nil {
		t.Fatalf("ListInDir --unmerged failed: %v", err)
	}
	outUnmerged := bufUnmerged.String()
	if !strings.Contains(outUnmerged, "bws-agent-wip-2") {
		t.Errorf("expected bws-agent-wip-2 in --unmerged output, got:\n%s", outUnmerged)
	}
	if strings.Contains(outUnmerged, "bws-agent-merged-1") {
		t.Errorf("did not expect merged branch in --unmerged output, got:\n%s", outUnmerged)
	}
}
