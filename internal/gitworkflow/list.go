package gitworkflow

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
)

// BranchInfo holds metadata about a bws-agent branch.
type BranchInfo struct {
	Name          string
	Hash          string
	CommitterDate string
	Author        string
	Subject       string
	Merged        bool
	IsCurrent     bool
}

// ListOptions specifies filtering options for listing branches.
type ListOptions struct {
	MergedOnly   bool
	UnmergedOnly bool
	Writer       io.Writer
}

// List prints all bws-agent-* branches in the current git repository.
func List(opts ListOptions) error {
	return ListInDir("", opts)
}

// ListInDir prints all bws-agent-* branches in the specified repository directory.
func ListInDir(dir string, opts ListOptions) error {
	w := opts.Writer
	if w == nil {
		w = os.Stdout
	}

	repoRoot, err := getGitRootDir(dir)
	if err != nil {
		return fmt.Errorf("not in a git repository: %w", err)
	}

	branches, err := GetAgentBranches(repoRoot)
	if err != nil {
		return fmt.Errorf("listing agent branches: %w", err)
	}

	var filtered []BranchInfo
	for _, b := range branches {
		if opts.MergedOnly && !b.Merged {
			continue
		}
		if opts.UnmergedOnly && b.Merged {
			continue
		}
		filtered = append(filtered, b)
	}

	if len(filtered) == 0 {
		if opts.MergedOnly {
			fmt.Fprintln(w, "No merged bws-agent branches found.")
		} else if opts.UnmergedOnly {
			fmt.Fprintln(w, "No unmerged bws-agent branches found.")
		} else {
			fmt.Fprintln(w, "No bws-agent branches found.")
		}
		return nil
	}

	return PrintBranches(w, filtered)
}

// GetAgentBranches retrieves all bws-agent-* branches and their commit info.
func GetAgentBranches(repoDir string) ([]BranchInfo, error) {
	format := "%(refname:short)%09%(objectname:short)%09%(committerdate:relative)%09%(authorname)%09%(contents:subject)"
	out, err := runCmdOutput(repoDir, "git", "for-each-ref", "--sort=-committerdate", fmt.Sprintf("--format=%s", format), "refs/heads/bws-agent-*")
	if err != nil {
		return nil, err
	}

	currentBranch, _ := getCurrentBranch(repoDir)

	lines := strings.Split(strings.TrimSpace(out), "\n")
	var branches []BranchInfo
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}
		name := parts[0]
		hash := parts[1]
		date := ""
		if len(parts) > 2 {
			date = parts[2]
		}
		author := ""
		if len(parts) > 3 {
			author = parts[3]
		}
		subject := ""
		if len(parts) > 4 {
			subject = parts[4]
		}

		merged := false
		if err := runCmd(repoDir, "git", "merge-base", "--is-ancestor", name, "HEAD"); err == nil {
			merged = true
		}

		branches = append(branches, BranchInfo{
			Name:          name,
			Hash:          hash,
			CommitterDate: date,
			Author:        author,
			Subject:       subject,
			Merged:        merged,
			IsCurrent:     (name == currentBranch),
		})
	}

	return branches, nil
}

// PrintBranches formats and prints the branch table.
func PrintBranches(w io.Writer, branches []BranchInfo) error {
	cyan := color.New(color.FgCyan, color.Bold).SprintFunc()
	dim := color.New(color.FgHiBlack).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()

	maxNameLen := 12
	for _, b := range branches {
		nameLen := len(b.Name)
		if b.IsCurrent {
			nameLen += 2
		}
		if nameLen > maxNameLen {
			maxNameLen = nameLen
		}
	}
	if maxNameLen > 40 {
		maxNameLen = 40
	}

	fmt.Fprintf(w, "%-*s  %-7s  %-16s  %-10s  %s\n", maxNameLen, "BRANCH", "COMMIT", "UPDATED", "STATUS", "SUBJECT")

	for _, b := range branches {
		displayName := b.Name
		if b.IsCurrent {
			displayName = "* " + displayName
		}
		coloredName := cyan(displayName)
		if len(displayName) < maxNameLen {
			padding := strings.Repeat(" ", maxNameLen-len(displayName))
			coloredName = coloredName + padding
		}

		statusStr := "[unmerged]"
		if b.Merged {
			statusStr = green("[merged]  ")
		} else {
			statusStr = yellow("[unmerged]")
		}

		commitHash := dim(b.Hash)
		if len(b.Hash) > 7 {
			commitHash = dim(b.Hash[:7])
		}

		fmt.Fprintf(w, "%s  %-7s  %-16s  %s  %s\n",
			coloredName,
			commitHash,
			dim(fmt.Sprintf("%-16s", b.CommitterDate)),
			statusStr,
			b.Subject,
		)
	}

	return nil
}
