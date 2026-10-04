package gitworkflow

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"bws/internal/util"
)

// PruneOptions defines options for pruning agent branches and temp directories.
type PruneOptions struct {
	All     bool
	DryRun  bool
	Verbose bool
	TempDir string
	Writer  io.Writer
}

// PruneResult records the outcomes of a prune operation.
type PruneResult struct {
	PrunedBranches  []string
	SkippedBranches []string
	PrunedDirs      []string
}

// Prune deletes merged (or all if All=true) bws-agent branches and /tmp/bws/agent_* temp dirs.
func Prune(opts PruneOptions) error {
	return PruneInDir("", opts)
}

// PruneInDir deletes merged (or all if All=true) bws-agent branches and temp dirs for a given repo dir.
func PruneInDir(dir string, opts PruneOptions) error {
	w := opts.Writer
	if w == nil {
		w = os.Stdout
	}

	result := &PruneResult{}

	repoRoot, err := getGitRootDir(dir)
	if err == nil {
		pruneBranches(repoRoot, opts, result)
	} else if opts.Verbose {
		fmt.Fprintf(os.Stderr, "[verbose] Not in a git repo: %v\n", err)
	}

	var baseDirs []string
	if opts.TempDir != "" {
		baseDirs = []string{opts.TempDir}
	} else {
		baseDirs = []string{util.UserTempDirBase(), "/tmp/bws"}
	}

	pruneTempDirs(baseDirs, repoRoot, opts, result)
	printPruneReport(w, result, opts, strings.Join(baseDirs, ", "))
	return nil
}

func pruneBranches(repoRoot string, opts PruneOptions, result *PruneResult) {
	currentBranch, _ := getCurrentBranch(repoRoot)
	branches, err := GetAgentBranches(repoRoot)
	if err != nil {
		return
	}
	for _, b := range branches {
		if b.Name == currentBranch {
			if opts.Verbose {
				fmt.Fprintf(os.Stderr, "[verbose] Skipping current checked-out branch: %s\n", b.Name)
			}
			continue
		}

		if b.Merged || opts.All {
			if opts.DryRun {
				result.PrunedBranches = append(result.PrunedBranches, b.Name)
			} else if err := runCmd(repoRoot, "git", "branch", "-D", b.Name); err == nil {
				result.PrunedBranches = append(result.PrunedBranches, b.Name)
			} else if opts.Verbose {
				fmt.Fprintf(os.Stderr, "[verbose] Failed to delete branch %s: %v\n", b.Name, err)
			}
		} else {
			result.SkippedBranches = append(result.SkippedBranches, b.Name)
		}
	}
}

func isActiveDir(targetReal, cwdReal, repoRootReal, execPathReal string) bool {
	if cwdReal != "" && (targetReal == cwdReal || strings.HasPrefix(cwdReal, targetReal+string(filepath.Separator))) {
		return true
	}
	if repoRootReal != "" && (targetReal == repoRootReal || strings.HasPrefix(repoRootReal, targetReal+string(filepath.Separator))) {
		return true
	}
	if execPathReal != "" && (targetReal == filepath.Dir(execPathReal) || strings.HasPrefix(execPathReal, targetReal+string(filepath.Separator))) {
		return true
	}
	return false
}

func pruneTempDirs(baseDirs []string, repoRoot string, opts PruneOptions, result *PruneResult) {
	execPath, _ := os.Executable()
	execPathReal, _ := filepath.EvalSymlinks(execPath)
	cwd, _ := os.Getwd()
	cwdReal, _ := filepath.EvalSymlinks(cwd)
	repoRootReal, _ := filepath.EvalSymlinks(repoRoot)

	seen := make(map[string]bool)
	for _, baseTemp := range baseDirs {
		entries, err := os.ReadDir(baseTemp)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !strings.HasPrefix(e.Name(), "agent_") || !e.IsDir() {
				continue
			}
			targetPath := filepath.Join(baseTemp, e.Name())
			if seen[targetPath] {
				continue
			}
			seen[targetPath] = true
			targetReal, _ := filepath.EvalSymlinks(targetPath)
			if targetReal == "" {
				targetReal = targetPath
			}
			if isActiveDir(targetReal, cwdReal, repoRootReal, execPathReal) {
				if opts.Verbose {
					fmt.Fprintf(os.Stderr, "[verbose] Skipping active directory: %s\n", targetPath)
				}
				continue
			}
			if opts.DryRun {
				result.PrunedDirs = append(result.PrunedDirs, targetPath)
			} else if err := os.RemoveAll(targetPath); err == nil {
				result.PrunedDirs = append(result.PrunedDirs, targetPath)
			} else if opts.Verbose {
				fmt.Fprintf(os.Stderr, "[verbose] Failed to remove %s: %v\n", targetPath, err)
			}
		}
	}
}

func printPruneReport(w io.Writer, res *PruneResult, opts PruneOptions, baseTemp string) {
	if opts.DryRun {
		fmt.Fprintln(w, "[dry-run] The following resources would be pruned:")
	}

	actionBranch := "Pruned"
	actionDir := "Cleaned up"
	if opts.DryRun {
		actionBranch = "Would prune"
		actionDir = "Would clean up"
	}

	hasActions := false
	if len(res.PrunedBranches) > 0 {
		hasActions = true
		fmt.Fprintf(w, "%s %d branch(es):\n", actionBranch, len(res.PrunedBranches))
		for _, b := range res.PrunedBranches {
			fmt.Fprintf(w, "  - %s\n", b)
		}
	}

	if len(res.SkippedBranches) > 0 {
		fmt.Fprintf(w, "Skipped %d unmerged branch(es) (use -a / --all to prune):\n", len(res.SkippedBranches))
		for _, b := range res.SkippedBranches {
			fmt.Fprintf(w, "  - %s\n", b)
		}
	}

	if len(res.PrunedDirs) > 0 {
		hasActions = true
		fmt.Fprintf(w, "%s %d temp directory(ies) in %s:\n", actionDir, len(res.PrunedDirs), baseTemp)
		for _, d := range res.PrunedDirs {
			fmt.Fprintf(w, "  - %s\n", d)
		}
	}

	if !hasActions {
		if len(res.SkippedBranches) > 0 {
			fmt.Fprintln(w, "No merged bws-agent branches or temp directories to prune (use -a / --all to prune unmerged branches).")
		} else {
			fmt.Fprintln(w, "No bws-agent branches or temp directories to prune.")
		}
	}
}
