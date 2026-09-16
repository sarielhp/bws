package cli

import (
	"bufio"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"bws/internal/config"
	"bws/internal/policy"

	"golang.org/x/term"
)

// AutoConfigureWorkspace checks targetDir for project stacks. If found, it generates
// and writes .bws/config.jsonc and returns (configPath, detectedSummary, nil).
// If no project features are detected, it returns ("", "", nil) without writing any file.
func AutoConfigureWorkspace(targetDir string, noSSH bool) (string, string, error) {
	root, before, err := initDestination(targetDir)
	if err != nil {
		return "", "", err
	}
	if before != nil {
		return "", "", nil
	}
	names, err := basicProfiles(root)
	if err != nil {
		return "", "", err
	}
	if len(names) == 0 {
		return "", "", nil
	}
	features, err := config.DetectFeatures(root)
	if err != nil {
		return "", "", err
	}
	summary := strings.Join(features.DetectedStacks(), ", ")
	if summary == "" {
		summary = strings.Join(names, ", ")
	}
	plan, err := BuildInitPlan(root, names, policy.Flags{NoSSH: noSSH})
	if err != nil {
		return "", "", err
	}
	path := filepath.Join(root, ".bws", "config.jsonc")
	if err := config.AtomicPolicyWrite(path, plan.Data, nil); err != nil {
		return "", "", err
	}
	return path, summary, nil
}

// PromptAutoInit asks the user if they want to auto-configure the workspace.
// Returns true for affirmative ("y", "yes", or empty Enter), false otherwise.
func PromptAutoInit(r io.Reader, w io.Writer) (bool, error) {
	fmt.Fprintf(w, "[bws] No .bws/config.jsonc found. Auto-configure workspace now? [Y/n] ")
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		return false, scanner.Err()
	}
	ans := strings.TrimSpace(scanner.Text())
	if ans == "" || strings.EqualFold(ans, "y") || strings.EqualFold(ans, "yes") {
		return true, nil
	}
	return false, nil
}

// IsInteractiveTTY checks if the file descriptor refers to a terminal.
func IsInteractiveTTY(fd int) bool {
	return term.IsTerminal(fd)
}
