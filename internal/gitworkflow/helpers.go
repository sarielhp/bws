package gitworkflow

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func getGitRootDir(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func getGitRoot() (string, error) {
	return getGitRootDir("")
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

func runCmdOutput(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return string(out), err
}

func copyConfigFiles(srcDir, destDir string) {
	srcBws := filepath.Join(srcDir, ".bws")
	if fi, err := os.Stat(srcBws); err == nil && fi.IsDir() {
		if err := copyDirRecursive(srcBws, filepath.Join(destDir, ".bws")); err != nil {
			// explicitly ignored
		}
	}
	// Copy .env files
	entries, _ := os.ReadDir(srcDir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".env") && !e.IsDir() {
			data, err := os.ReadFile(filepath.Join(srcDir, e.Name()))
			if err == nil {
				if err := os.WriteFile(filepath.Join(destDir, e.Name()), data, 0644); err != nil {
					// explicitly ignored
				}
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
