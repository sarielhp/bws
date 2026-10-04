package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BackupSuffix is appended to a configuration path to form its single-slot
// backup file, e.g. .bws/config.jsonc -> .bws/config.jsonc.bak.
const BackupSuffix = ".bak"

// BackupPath returns the depth-1 backup path for a configuration file.
func BackupPath(cfgPath string) string {
	return cfgPath + BackupSuffix
}

// IsConfigFile reports whether a path names a bws configuration file (global
// or workspace), excluding profile files. Backups apply to configuration only.
func IsConfigFile(path string) bool {
	path = filepath.Clean(path)
	base := filepath.Base(path)
	if path == filepath.Clean(GlobalPath()) {
		return true
	}
	if base == ".bws.jsonc" {
		return true
	}
	if (base == "config.jsonc" || base == "config.json") &&
		filepath.Base(filepath.Dir(path)) == ".bws" {
		return true
	}
	return false
}

// resolvedConfigPath ensures paths are clean and never follow symlinks.
func resolvedConfigPath(path string) string {
	return filepath.Clean(path)
}

// BackupConfig records the current contents of a configuration file as its
// single-slot backup. Non-config paths and absent files are ignored.
func BackupConfig(path string) error {
	return backupConfig(path)
}

// backupConfig copies the current contents of a configuration file to its
// single-slot backup before a write. The backup always holds the state
// immediately before the most recent bws write. Non-config paths and absent
// files are ignored. The copy is written atomically and fsynced.
func backupConfig(path string) error {
	if !IsConfigFile(path) {
		return nil
	}
	fi, err := os.Lstat(path)
	if err == nil && fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to backup through symlink: %s", path)
	}
	resolved := resolvedConfigPath(path)
	data, err := os.ReadFile(resolved)
	if os.IsNotExist(err) {
		// First write: there is no prior state to preserve. Remove any stale
		// backup so an undo cannot restore a file that predates creation.
		_ = os.Remove(BackupPath(resolved))
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading config for backup: %w", err)
	}
	if err := atomicWriteFile(BackupPath(resolved), data); err != nil {
		return fmt.Errorf("writing config backup: %w", err)
	}
	return nil
}

// HasBackup reports whether a restorable backup exists for a configuration.
func HasBackup(path string) bool {
	resolved := resolvedConfigPath(path)
	fi, err := os.Stat(BackupPath(resolved))
	return err == nil && fi.Mode().IsRegular()
}

// RestoreBackup restores a configuration from its single-slot backup. The
// restored bytes are intentionally written without recording trust, so a local
// workspace must be re-reviewed with 'bws config trust' after an undo. The
// backup is retained, making undo idempotent.
func RestoreBackup(path string) error {
	if !IsConfigFile(path) {
		return fmt.Errorf("not a bws configuration file: %s", path)
	}
	fi, err := os.Lstat(path)
	if err == nil && fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to restore through symlink: %s", path)
	}
	resolved := resolvedConfigPath(path)
	backup := BackupPath(resolved)
	data, err := os.ReadFile(backup)
	if os.IsNotExist(err) {
		return fmt.Errorf("no backup available for %s", displayConfigPath(resolved))
	}
	if err != nil {
		return fmt.Errorf("reading config backup: %w", err)
	}
	if err := atomicWriteFile(resolved, data); err != nil {
		return fmt.Errorf("restoring config backup: %w", err)
	}
	return nil
}

// DisplayPath shortens a home-anchored path to '~/...' for user-facing messages.
func DisplayPath(path string) string {
	return displayConfigPath(path)
}

// displayConfigPath shortens a home-anchored path to '~/...' for messages.
func displayConfigPath(path string) string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
