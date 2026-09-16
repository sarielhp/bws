package gitworkflow

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"bws/internal/config"
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

func copyConfigFiles(srcDir, destDir string) error {
	src, err := os.OpenRoot(srcDir)
	if err != nil {
		return err
	}
	defer src.Close()
	dest, err := os.OpenRoot(destDir)
	if err != nil {
		return err
	}
	defer dest.Close()
	entries, err := fs.ReadDir(src.FS(), ".")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == ".bws" {
			if err := copyPolicyTree(src, dest, name); err != nil {
				return err
			}
		} else if name == ".bws.jsonc" || (strings.HasPrefix(name, ".env") && !entry.IsDir()) {
			if err := copyPolicyFile(src, dest, name); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyPolicyTree(src, dest *os.Root, dir string) error {
	return fs.WalkDir(src.FS(), dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return dest.MkdirAll(path, 0755)
		}
		return copyPolicyFile(src, dest, path)
	})
}

func copyPolicyFile(src, dest *os.Root, path string) error {
	info, err := src.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("refusing non-regular workspace configuration %s", path)
	}
	data, err := src.ReadFile(path)
	if err != nil {
		return err
	}
	if config.IsLocalPolicy(path) {
		data, err = config.ReadTrustedFile(filepath.Join(src.Name(), path))
		if err != nil {
			return err
		}
	}
	if err := dest.WriteFile(path, data, 0644); err != nil {
		return err
	}
	if config.IsLocalPolicy(path) {
		return config.TrustFile(filepath.Join(dest.Name(), path))
	}
	return nil
}

func excludeSensitiveFiles(destDir string) {
	excludePath := filepath.Join(destDir, ".git", "info", "exclude")
	f, err := os.OpenFile(excludePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		defer f.Close()
		_, _ = f.WriteString("\n.bws/\n.env*\n")
	}
}
