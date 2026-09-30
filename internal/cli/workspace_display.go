package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"bws/internal/config"

	"github.com/fatih/color"
)

// FormatWorkspace returns the colored workspace path, followed by a yellow
// "(not current directory)" notice if the path is not the current working directory.
func FormatWorkspace(wsRoot string) string {
	if wsRoot == "" {
		cwd, _ := os.Getwd()
		wsRoot, _ = config.FindWorkspaceRoot(cwd)
	}
	cwd, _ := os.Getwd()

	isCurrent := isSameDirectory(wsRoot, cwd)
	coloredPath := color.New(color.FgCyan, color.Bold).Sprint(wsRoot)
	if !isCurrent {
		yellowNotice := color.New(color.FgYellow).Sprint("(not current directory)")
		return fmt.Sprintf("%s %s", coloredPath, yellowNotice)
	}
	return coloredPath
}

// FormatWorkspaceHeader returns "Workspace: " followed by FormatWorkspace(wsRoot).
func FormatWorkspaceHeader(wsRoot string) string {
	return fmt.Sprintf("Workspace: %s", FormatWorkspace(wsRoot))
}

// PrintWorkspaceInfo prints "Workspace: <coloredPath> [(not current directory)]".
func PrintWorkspaceInfo(wsRoot string) {
	fmt.Println(FormatWorkspaceHeader(wsRoot))
}

func isSameDirectory(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	cleanA := filepath.Clean(a)
	cleanB := filepath.Clean(b)
	if cleanA == cleanB {
		return true
	}
	realA, errA := filepath.EvalSymlinks(cleanA)
	realB, errB := filepath.EvalSymlinks(cleanB)
	if errA == nil && errB == nil && realA == realB {
		return true
	}
	return false
}

func findWorkspaceForPath(targetPath string) string {
	if targetPath != "" {
		abs, err := filepath.Abs(targetPath)
		if err == nil {
			dir := filepath.Dir(abs)
			if filepath.Base(dir) == ".bws" {
				return filepath.Dir(dir)
			}
			return dir
		}
	}
	cwd, _ := os.Getwd()
	root, _ := config.FindWorkspaceRoot(cwd)
	return root
}
