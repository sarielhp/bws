package learn

import (
	"errors"
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
	var errs []error
	if delta == nil || delta.IsEmpty() {
		return res, nil
	}

	// 1. Remove upgraded RO entries. The entry may live in another config
	// file, so a miss is not an error.
	for _, oldRO := range delta.UpgradedRO {
		if found, err := config.RemoveBindElement(targetPath, "binds_ro", oldRO); err == nil && found {
			res.UpgradedRO++
		}
	}

	// 2. Add new RW mounts
	for _, rw := range delta.BindsRW {
		entry := fmt.Sprintf("%q", rw)
		if err := config.AddBindArrayElement(targetPath, "binds_rw", entry); err != nil {
			errs = append(errs, err)
		} else {
			res.AddedRW++
		}
	}

	// 3. Add new RO mounts
	for _, ro := range delta.BindsRO {
		entry := fmt.Sprintf("%q", ro)
		if err := config.AddBindArrayElement(targetPath, "binds_ro", entry); err != nil {
			errs = append(errs, err)
		} else {
			res.AddedRO++
		}
	}

	// 4. Add new PATH entries
	for _, p := range delta.Path {
		if err := config.AddArrayElement(targetPath, "path", p); err != nil {
			errs = append(errs, err)
		} else {
			res.AddedPath++
		}
	}

	// 5. Enable detected features
	errs = append(errs, enableFeatures(targetPath, delta.Features, res)...)

	return res, errors.Join(errs...)
}

func enableFeatures(targetPath string, f DetectedFeatures, res *MergeResult) []error {
	var errs []error
	for _, feat := range []struct {
		on  bool
		key string
	}{{f.SSH, "enable_ssh"}, {f.DBus, "enable_dbus"}, {f.X11, "enable_x11"}, {f.WSL, "enable_wsl"}} {
		if !feat.on {
			continue
		}
		if err := config.SetConfigKV(targetPath, feat.key, "true"); err != nil {
			errs = append(errs, err)
			continue
		}
		res.EnabledFeatures = append(res.EnabledFeatures, feat.key)
	}
	return errs
}
