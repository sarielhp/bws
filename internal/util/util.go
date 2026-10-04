package util

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

func CommandExists(cmd string) bool {
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		p := filepath.Join(dir, cmd)
		if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() && fi.Mode()&0111 != 0 {
			return true
		}
	}
	return false
}

func EnsureBwrap() error {
	if CommandExists("bwrap") {
		return nil
	}
	return fmt.Errorf("'bwrap' (Bubblewrap) executable not found in PATH.\n\n" +
		"bws is a declarative wrapper and launcher for Bubblewrap:\n" +
		"  https://github.com/containers/bubblewrap\n\n" +
		"Installation commands:\n" +
		"  Debian / Ubuntu / Mint:  sudo apt install bubblewrap\n" +
		"  Fedora / RHEL:           sudo dnf install bubblewrap\n" +
		"  Arch Linux:              sudo pacman -S bubblewrap\n" +
		"  Alpine Linux:            sudo apk add bubblewrap\n" +
		"  openSUSE:                sudo zypper install bubblewrap")
}

var junkDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"target":       true,
	"vendor":       true,
	"__pycache__":  true,
	".cache":       true,
	".bws":         true,
	".bw":          true,
}

func CountFiles(dir string, limit int) int {
	count := 0
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return filepath.SkipDir
		}
		if d.IsDir() && junkDirs[d.Name()] {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			count++
			if count > limit {
				return filepath.SkipAll
			}
		}
		return nil
	})
	return count
}

func CopyIfNewer(src, dest string) (copied bool) {
	fi, err := os.Stat(src)
	if err != nil || !fi.Mode().IsRegular() {
		return false
	}
	os.MkdirAll(filepath.Dir(dest), 0755)
	dfi, err := os.Stat(dest)
	if err != nil || fi.ModTime().After(dfi.ModTime()) {
		if err := copyFile(src, dest); err == nil {
			return true
		}
	}
	return false
}

func copyFile(src, dest string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0644)
}

// HomeDir returns the home directory from $HOME, falling back to the passwd
// entry so an unset $HOME never yields a CWD-relative path.
func HomeDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	if u, err := user.Current(); err == nil && filepath.IsAbs(u.HomeDir) {
		return u.HomeDir
	}
	return "/"
}

func ExpandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(HomeDir(), path[2:])
	}
	if path == "~" {
		return HomeDir()
	}
	return path
}

// UserTempDirBase returns the base per-user temporary directory path /tmp/bws-<UID>.
func UserTempDirBase() string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("bws-%d", os.Getuid()))
}

// UserTempDir creates and returns an ephemeral temporary directory scoped to the
// current user's UID (e.g. /tmp/bws-<UID>/<sub_*>), falling back to os.TempDir()/bws_<sub_*>
// if the per-user base cannot be created or accessed with appropriate permissions.
func UserTempDir(sub string) (string, error) {
	base := UserTempDirBase()
	if err := os.MkdirAll(base, 0700); err == nil {
		if fi, err := os.Stat(base); err == nil && fi.IsDir() && fi.Mode().Perm()&0077 == 0 {
			if tmp, err := os.MkdirTemp(base, sub+"_"); err == nil {
				return tmp, nil
			}
		}
	}
	return os.MkdirTemp("", "bws_"+sub+"_")
}
