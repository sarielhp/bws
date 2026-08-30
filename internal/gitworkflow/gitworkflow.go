package gitworkflow

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Options defines configuration for a git-workflow session.
type Options struct {
	Branch     string
	AllowDirty bool
	Stash      bool
	Command    []string
	Verbose    bool
}

// Run executes the full Clone-Fetch agent workflow.
func Run(opts Options) error {
	hostRepo, err := getGitRoot()
	if err != nil {
		return fmt.Errorf("not in a git repository: %w", err)
	}

	baseBranch, err := getCurrentBranch(hostRepo)
	if err != nil {
		return fmt.Errorf("determining current branch: %w", err)
	}

	isDirty, err := checkDirty(hostRepo)
	if err != nil {
		return fmt.Errorf("checking git status: %w", err)
	}

	stashed := false
	if isDirty {
		if opts.Stash {
			if err := runCmd(hostRepo, "git", "stash", "push", "-m", "bws-git-workflow-auto-stash"); err != nil {
				return fmt.Errorf("stashing working tree: %w", err)
			}
			stashed = true
			defer func() {
				if stashed {
					_ = runCmd(hostRepo, "git", "stash", "pop")
				}
			}()
		} else if !opts.AllowDirty {
			return fmt.Errorf("working tree has uncommitted changes. Commit, stash, or pass --stash / --allow-dirty")
		}
	}

	_ = os.MkdirAll("/tmp/bws", 0755)
	tempDir, err := os.MkdirTemp("/tmp/bws", "agent_")
	if err != nil {
		return fmt.Errorf("creating agent temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	branchName := opts.Branch
	if branchName == "" {
		branchName = fmt.Sprintf("bws-agent-%s", time.Now().Format("20060102-150405"))
	}

	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "[verbose] Ephemeral agent clone path: %s\n", tempDir)
		fmt.Fprintf(os.Stderr, "[verbose] Ephemeral agent branch: %s\n", branchName)
	}

	// Clone repo locally with object sharing
	if err := runCmd(hostRepo, "git", "clone", "--shared", fmt.Sprintf("file://%s", hostRepo), tempDir); err != nil {
		return fmt.Errorf("cloning to agent workspace: %w", err)
	}

	// Create and switch to new branch in clone
	if err := runCmd(tempDir, "git", "checkout", "-b", branchName); err != nil {
		return fmt.Errorf("checking out branch in agent workspace: %w", err)
	}

	// Copy .bws workspace config and .env if present
	copyConfigFiles(hostRepo, tempDir)

	// Execute bws inside the clone
	fmt.Printf("\n=== Entering Bubblewrap Agent Sandbox (%s) ===\n", branchName)
	bwsBin, err := os.Executable()
	if err != nil || strings.Contains(bwsBin, "go-build") {
		bwsBin = "bws"
	}

	bwsArgs := []string{"run", "--no-ssh"}
	if opts.Verbose {
		bwsArgs = append(bwsArgs, "-v")
	}
	if len(opts.Command) > 0 {
		bwsArgs = append(bwsArgs, "--")
		bwsArgs = append(bwsArgs, opts.Command...)
	}

	cmd := exec.Command(bwsBin, bwsArgs...)
	cmd.Dir = tempDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()

	fmt.Printf("\n=== Sandbox Session Ended ===\n")

	// Check if there are changes in the clone
	cloneDirty, _ := checkDirty(tempDir)
	if cloneDirty {
		fmt.Println("Auto-committing remaining changes in agent workspace...")
		_ = runCmd(tempDir, "git", "add", "-A")
		_ = runCmd(tempDir, "git", "commit", "-m", fmt.Sprintf("bws(agent): changes from session on %s", branchName))
	}

	// Fetch the branch back to the host
	if err := runCmd(hostRepo, "git", "fetch", tempDir, fmt.Sprintf("%s:%s", branchName, branchName)); err != nil {
		fmt.Printf("Note: No new commits or branch changes detected from agent session.\n")
		return nil
	}

	// Show diff summary
	diffStat, _ := getDiffStat(hostRepo, baseBranch, branchName)
	if strings.TrimSpace(diffStat) == "" {
		fmt.Printf("No changes between %s and %s.\n", baseBranch, branchName)
		_ = runCmd(hostRepo, "git", "branch", "-D", branchName)
		return nil
	}

	fmt.Printf("\nAgent changes on branch %s:\n\n%s\n", branchName, diffStat)

	// Interactive triage prompt
	promptTriage(hostRepo, baseBranch, branchName)
	return nil
}

func promptTriage(hostRepo, baseBranch, branchName string) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("\nWhat would you like to do with branch %q?\n", branchName)
		fmt.Println("  [m] Merge   - Fast-forward or merge into current branch")
		fmt.Println("  [s] Squash  - Squash all commits into one commit on current branch")
		fmt.Println("  [k] Keep    - Keep branch for manual inspection")
		fmt.Println("  [d] Discard - Delete branch and discard all agent changes")
		fmt.Println("  [v] View    - Open full diff in pager")
		fmt.Print("> ")

		input, _ := reader.ReadString('\n')
		choice := strings.ToLower(strings.TrimSpace(input))

		switch choice {
		case "m", "merge":
			if err := runCmd(hostRepo, "git", "merge", branchName); err != nil {
				fmt.Fprintf(os.Stderr, "Merge encountered conflicts or failed: %v\n", err)
				fmt.Println("Keeping branch for manual resolution.")
				return
			}
			_ = runCmd(hostRepo, "git", "branch", "-D", branchName)
			fmt.Printf("Merged %s into %s and removed temporary branch.\n", branchName, baseBranch)
			return

		case "s", "squash":
			if err := runCmd(hostRepo, "git", "merge", "--squash", branchName); err != nil {
				fmt.Fprintf(os.Stderr, "Squash merge failed: %v\n", err)
				return
			}
			_ = runCmd(hostRepo, "git", "commit", "-m", fmt.Sprintf("bws(agent): squash changes from %s", branchName))
			_ = runCmd(hostRepo, "git", "branch", "-D", branchName)
			fmt.Printf("Squash-merged %s into %s and committed changes.\n", branchName, baseBranch)
			return

		case "k", "keep":
			fmt.Printf("Preserved branch %q on host repository.\n", branchName)
			return

		case "d", "discard":
			_ = runCmd(hostRepo, "git", "branch", "-D", branchName)
			fmt.Printf("Discarded and deleted branch %q.\n", branchName)
			return

		case "v", "view":
			pager := os.Getenv("PAGER")
			if pager == "" {
				pager = "less"
			}
			viewCmd := exec.Command("git", "diff", fmt.Sprintf("%s..%s", baseBranch, branchName))
			viewCmd.Dir = hostRepo
			viewCmd.Stdout = os.Stdout
			viewCmd.Stderr = os.Stderr
			_ = viewCmd.Run()

		default:
			fmt.Println("Invalid choice. Please enter m, s, k, d, or v.")
		}
	}
}

func getGitRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func getCurrentBranch(dir string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func checkDirty(dir string) (bool, error) {
	out, err := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	if err != nil {
		return false, err
	}
	return len(strings.TrimSpace(string(out))) > 0, nil
}

func getDiffStat(dir, base, target string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "diff", "--stat", fmt.Sprintf("%s..%s", base, target)).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func runCmd(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return cmd.Run()
}

func copyConfigFiles(srcDir, destDir string) {
	srcBws := filepath.Join(srcDir, ".bws")
	if fi, err := os.Stat(srcBws); err == nil && fi.IsDir() {
		_ = copyDirRecursive(srcBws, filepath.Join(destDir, ".bws"))
	}
	// Copy .env files
	entries, _ := os.ReadDir(srcDir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".env") && !e.IsDir() {
			data, err := os.ReadFile(filepath.Join(srcDir, e.Name()))
			if err == nil {
				_ = os.WriteFile(filepath.Join(destDir, e.Name()), data, 0644)
			}
		}
	}
}

func copyDirRecursive(src, dest string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	})
}
