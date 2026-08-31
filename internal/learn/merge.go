package learn

import (
	"fmt"
	"os"

	"bws/internal/config"
)

// MergeResult summarizes the modifications made during live config merging.
type MergeResult struct {
	AddedRW         int
	AddedRO         int
	UpgradedRO      int
	AddedPath       int
	EnabledFeatures []string
}

// ApplyDelta updates the target JSONC configuration with the discovered delta.
func ApplyDelta(targetPath string, delta *Delta) (*MergeResult, error) {
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		if err := config.CreateDefault(targetPath); err != nil {
			return nil, fmt.Errorf("creating default config at %s: %w", targetPath, err)
		}
	}

	res := &MergeResult{}
	if delta == nil || delta.IsEmpty() {
		return res, nil
	}

	// 1. Remove upgraded RO entries
	for _, oldRO := range delta.UpgradedRO {
		if found, err := config.RemoveBindElement(targetPath, "binds_ro", oldRO); err == nil && found {
			res.UpgradedRO++
		}
	}

	// 2. Add new RW mounts
	for _, rw := range delta.BindsRW {
		entry := fmt.Sprintf("%q", rw)
		if err := config.AddBindArrayElement(targetPath, "binds_rw", entry); err == nil {
			res.AddedRW++
		}
	}

	// 3. Add new RO mounts
	for _, ro := range delta.BindsRO {
		entry := fmt.Sprintf("%q", ro)
		if err := config.AddBindArrayElement(targetPath, "binds_ro", entry); err == nil {
			res.AddedRO++
		}
	}

	// 4. Add new PATH entries
	for _, p := range delta.Path {
		if err := config.AddArrayElement(targetPath, "path", p); err == nil {
			res.AddedPath++
		}
	}

	// 5. Enable detected features
	if delta.Features.SSH {
		if err := config.SetConfigKV(targetPath, "enable_ssh", "true"); err == nil {
			res.EnabledFeatures = append(res.EnabledFeatures, "enable_ssh")
		}
	}
	if delta.Features.DBus {
		if err := config.SetConfigKV(targetPath, "enable_dbus", "true"); err == nil {
			res.EnabledFeatures = append(res.EnabledFeatures, "enable_dbus")
		}
	}
	if delta.Features.X11 {
		if err := config.SetConfigKV(targetPath, "enable_x11", "true"); err == nil {
			res.EnabledFeatures = append(res.EnabledFeatures, "enable_x11")
		}
	}
	if delta.Features.WSL {
		if err := config.SetConfigKV(targetPath, "enable_wsl", "true"); err == nil {
			res.EnabledFeatures = append(res.EnabledFeatures, "enable_wsl")
		}
	}

	return res, nil
}
