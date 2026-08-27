package util

import (
	"os"
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

var junkDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"target":       true,
	"vendor":       true,
	"__pycache__":  true,
	".cache":       true,
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

func HomeDir() string {
	home, _ := os.UserHomeDir()
	return home
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
