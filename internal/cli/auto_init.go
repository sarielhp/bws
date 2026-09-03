package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"bws/internal/config"

	"golang.org/x/term"
)

// AutoConfigureWorkspace checks targetDir for project stacks. If found, it generates
// and writes .bws/config.jsonc and returns (configPath, detectedSummary, nil).
// If no project features are detected, it returns ("", "", nil) without writing any file.
func AutoConfigureWorkspace(targetDir string, noSSH bool) (string, string, error) {
	absDir, err := filepath.Abs(targetDir)
	if err != nil {
		return "", "", fmt.Errorf("resolving target directory: %w", err)
	}

	features, err := config.DetectFeatures(absDir)
	if err != nil {
		return "", "", fmt.Errorf("detecting workspace features: %w", err)
	}
	if !features.AnyDetected() {
		return "", "", nil
	}

	summary := strings.Join(features.DetectedStacks(), ", ")
	activeProfiles, extraRW, extraRO, extraPath, extraEnv := resolveInitProfiles(absDir, nil)

	opts := config.InitDevOptions{
		Features:     features,
		TargetDir:    absDir,
		Force:        false,
		DryRun:       false,
		NoSSH:        noSSH,
		Profiles:     activeProfiles,
		ExtraBindsRW: extraRW,
		ExtraBindsRO: extraRO,
		ExtraPath:    extraPath,
		ExtraEnv:     extraEnv,
	}

	jsonContent, err := config.GenerateDevConfigJSON(opts)
	if err != nil {
		return "", "", fmt.Errorf("generating dev configuration: %w", err)
	}

	configPath := filepath.Join(absDir, ".bws", "config.jsonc")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return "", "", fmt.Errorf("creating directory %s: %w", filepath.Dir(configPath), err)
	}
	if err := os.WriteFile(configPath, []byte(jsonContent), 0644); err != nil {
		return "", "", fmt.Errorf("writing configuration to %s: %w", configPath, err)
	}

	return configPath, summary, nil
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
