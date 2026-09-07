package gitworkflow

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
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
	NoNet      bool
	Proxy      bool
	NoProxy    bool
	DBus       bool
	NoDBus     bool
	NoInit     bool
}

func prepareHostRepo(opts Options) (string, string, string, func(), error) {
	hostRepo, err := getGitRoot()
	if err != nil {
		return "", "", "", nil, fmt.Errorf("not in a git repository: %w", err)
	}

	baseBranch, err := getCurrentBranch(hostRepo)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("determining current branch: %w", err)
	}
	if baseBranch == "HEAD" {
		return "", "", "", nil, fmt.Errorf("host repository is in detached HEAD state; checkout a branch before running git-workflow")
	}

	baseSHA, err := runCmdOutput(hostRepo, "git", "rev-parse", "HEAD")
	if err != nil {
		return "", "", "", nil, fmt.Errorf("determining base commit SHA: %w", err)
	}
	baseSHA = strings.TrimSpace(baseSHA)

	isDirty, err := checkDirty(hostRepo)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("checking git status: %w", err)
	}

	stashed := false
	if isDirty {
		if opts.Stash {
			if err := runCmd(hostRepo, "git", "stash", "push", "-m", "bws-git-workflow-auto-stash"); err != nil {
				return "", "", "", nil, fmt.Errorf("stashing working tree: %w", err)
			}
			stashed = true
		} else if !opts.AllowDirty {
			return "", "", "", nil, fmt.Errorf("working tree has uncommitted changes. Commit, stash, or pass --stash / --allow-dirty")
		}
	}

	unwrapStash := func() {
		if stashed {
			if err := runCmd(hostRepo, "git", "stash", "pop"); err != nil {
				fmt.Fprintf(os.Stderr, "\n[warning] Failed to pop automatic stash: %v\n", err)
				fmt.Fprintf(os.Stderr, "Your pre-session changes are preserved in stash (search for 'bws-git-workflow-auto-stash' with 'git stash list')\n")
			}
		}
	}

	return hostRepo, baseBranch, baseSHA, unwrapStash, nil
}

func checkoutAgentBranch(hostRepo, tempDir, branchName string) error {
	exists := false
	if out, err := exec.Command("git", "-C", hostRepo, "rev-parse", "--verify", fmt.Sprintf("refs/heads/%s", branchName)).Output(); err == nil && len(out) > 0 {
		exists = true
	}

	if exists {
		if err := runCmd(tempDir, "git", "checkout", branchName); err != nil {
			return fmt.Errorf("checking out existing branch in agent workspace: %w", err)
		}
	} else {
		if err := runCmd(tempDir, "git", "checkout", "-b", branchName); err != nil {
			return fmt.Errorf("checking out new branch in agent workspace: %w", err)
		}
	}
	return nil
}

func prepareClone(hostRepo, baseBranch, requestedBranch string, verbose bool) (string, string, func(), func(), error) {
	branchName := requestedBranch
	if branchName == "" {
		branchName = fmt.Sprintf("bws-agent-%s", time.Now().Format("20060102-150405"))
	}
	if branchName == baseBranch {
		return "", "", nil, nil, fmt.Errorf("cannot use current branch %q as agent branch; specify a different branch with -b", branchName)
	}

	if err := os.MkdirAll("/tmp/bws", 0755); err != nil {
		// explicitly ignored
	}
	tempDir, err := os.MkdirTemp("/tmp/bws", "agent_")
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("creating agent temp directory: %w", err)
	}

	keepDir := false
	cleanup := func() {
		if !keepDir {
			_ = os.RemoveAll(tempDir)
		}
	}
	cancelCleanup := func() {
		keepDir = true
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] Ephemeral agent clone path: %s\n", tempDir)
		fmt.Fprintf(os.Stderr, "[verbose] Ephemeral agent branch: %s\n", branchName)
	}

	if err := runCmd(hostRepo, "git", "clone", "--shared", fmt.Sprintf("file://%s", hostRepo), tempDir); err != nil {
		cleanup()
		return "", "", nil, nil, fmt.Errorf("cloning to agent workspace: %w", err)
	}

	if err := checkoutAgentBranch(hostRepo, tempDir, branchName); err != nil {
		cleanup()
		return "", "", nil, nil, err
	}

	copyConfigFiles(hostRepo, tempDir)
	excludeSensitiveFiles(tempDir)

	return tempDir, branchName, cleanup, cancelCleanup, nil
}

func buildBwsArgs(opts Options) []string {
	bwsArgs := []string{"run", "--no-ssh"}
	if opts.Verbose {
		bwsArgs = append(bwsArgs, "-v")
	}
	if opts.NoNet {
		bwsArgs = append(bwsArgs, "-N")
	}
	if opts.Proxy {
		bwsArgs = append(bwsArgs, "--proxy")
	}
	if opts.NoProxy {
		bwsArgs = append(bwsArgs, "--no-proxy")
	}
	if opts.DBus {
		bwsArgs = append(bwsArgs, "--dbus")
	}
	if opts.NoDBus {
		bwsArgs = append(bwsArgs, "--no-dbus")
	}
	if opts.NoInit {
		bwsArgs = append(bwsArgs, "--no-init")
	}
	if len(opts.Command) > 0 {
		bwsArgs = append(bwsArgs, "--")
		bwsArgs = append(bwsArgs, opts.Command...)
	}
	return bwsArgs
}

func runSandboxSession(tempDir string, opts Options, branchName string) error {
	fmt.Printf("\n=== Entering Bubblewrap Agent Sandbox (%s) ===\n", branchName)
	bwsBin, err := os.Executable()
	if err != nil || strings.Contains(bwsBin, "go-build") {
		bwsBin = "bws"
	}

	cmd := exec.Command(bwsBin, buildBwsArgs(opts)...)
	cmd.Dir = tempDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	runErr := cmd.Run()
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			fmt.Printf("\n[warning] Sandbox process exited with code %d\n", exitErr.ExitCode())
		} else {
			fmt.Printf("\n[warning] Sandbox process ended with error: %v\n", runErr)
		}
	}

	fmt.Printf("\n=== Sandbox Session Ended ===\n")
	return runErr
}

func commitAgentChanges(tempDir, branchName string) {
	cloneDirty, _ := checkDirty(tempDir)
	if cloneDirty {
		fmt.Println("Auto-committing remaining changes in agent workspace...")
		_ = runCmd(tempDir, "git", "add", "-A")
		_ = runCmd(tempDir, "git", "commit", "--no-verify", "-m", fmt.Sprintf("bws(agent): changes from session on %s", branchName))
	}
}

func fetchAgentBranch(hostRepo, tempDir, branchName string) error {
	if err := runCmd(hostRepo, "git", "fetch", tempDir, fmt.Sprintf("+%s:%s", branchName, branchName)); err != nil {
		return fmt.Errorf("failed to fetch agent branch %s back to host: %w", branchName, err)
	}
	return nil
}

func promptTriage(r io.Reader, hostRepo, baseSHA, baseBranch, branchName string) {
	if r == nil {
		r = os.Stdin
	}
	reader := bufio.NewReader(r)
	invalidAttempts := 0
	for {
		fmt.Printf("\nWhat would you like to do with branch %q?\n", branchName)
		fmt.Println("  [m] Merge        - Fast-forward or merge branch into current branch")
		fmt.Println("  [s] Squash-merge - Merge as a single commit into current branch")
		fmt.Println("  [k] Keep         - Keep branch for manual inspection without merging")
		fmt.Println("  [d] Discard      - Delete branch and discard all agent changes")
		fmt.Println("  [v] View         - Open full diff in pager")
		fmt.Print("> ")

		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("\nNon-interactive stdin or EOF encountered. Preserving branch %q for manual inspection.\n", branchName)
			return
		}
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

		case "s", "squash", "squash-merge":
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
			diffTarget := baseSHA
			if diffTarget == "" {
				diffTarget = baseBranch
			}
			viewCmd := exec.Command("git", "diff", fmt.Sprintf("%s..%s", diffTarget, branchName))
			viewCmd.Dir = hostRepo
			viewCmd.Stdout = os.Stdout
			viewCmd.Stderr = os.Stderr
			_ = viewCmd.Run()

		default:
			invalidAttempts++
			if invalidAttempts >= 3 {
				fmt.Printf("Too many invalid attempts. Preserving branch %q for manual inspection.\n", branchName)
				return
			}
			fmt.Println("Invalid choice. Please enter m, s, k, d, or v.")
		}
	}
}

// Run executes the full Clone-Fetch agent workflow.
func Run(opts Options) error {
	hostRepo, baseBranch, baseSHA, unwrapStash, err := prepareHostRepo(opts)
	if err != nil {
		return err
	}
	defer unwrapStash()

	tempDir, branchName, cleanupClone, cancelCleanup, err := prepareClone(hostRepo, baseBranch, opts.Branch, opts.Verbose)
	if err != nil {
		return err
	}
	defer cleanupClone()

	_ = runSandboxSession(tempDir, opts, branchName)

	commitAgentChanges(tempDir, branchName)

	if err := fetchAgentBranch(hostRepo, tempDir, branchName); err != nil {
		cancelCleanup()
		return fmt.Errorf("%w\nAgent workspace preserved at: %s", err, tempDir)
	}

	diffStat, _ := getDiffStat(hostRepo, baseSHA, branchName)
	if strings.TrimSpace(diffStat) == "" {
		fmt.Printf("No changes between %s and %s.\n", baseBranch, branchName)
		_ = runCmd(hostRepo, "git", "branch", "-D", branchName)
		return nil
	}

	fmt.Printf("\nAgent changes on branch %s:\n\n%s\n", branchName, diffStat)
	promptTriage(nil, hostRepo, baseSHA, baseBranch, branchName)
	return nil
}
