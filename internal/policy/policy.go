// Package policy resolves sandbox configuration without staging files or services.
package policy

import (
	"bws/internal/config"
	"fmt"
	"os"
	"path/filepath"
)

// Resolution includes effective policy and the unmodified configuration sources.
type Resolution struct {
	Config     *config.Config
	Global     *config.Config
	Local      *config.Config
	GlobalPath string
	LocalPath  string
	Workspace  string
}

// Load resolves the same policy used for launching, inspection and saving.
func Load(dir string) (*Resolution, error) {
	r, err := LoadSources(dir)
	if err != nil {
		return nil, err
	}
	r.Config, err = Resolve(r.Global, r.Local, r.Workspace)
	if err != nil {
		return nil, err
	}
	return r, nil
}

// LoadSources reads trusted source files without resolving previously selected profiles.
func LoadSources(dir string) (*Resolution, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	root, localPath := config.FindWorkspaceRoot(abs)
	globalPath := config.GlobalPath()
	global, err := config.LoadFile(globalPath)
	if os.IsNotExist(err) {
		global, err = config.Parse([]byte(config.DefaultConfigTemplate), globalPath)
	}
	if err != nil {
		return nil, fmt.Errorf("loading global config: %w", err)
	}
	var local *config.Config
	if _, statErr := os.Stat(localPath); statErr == nil {
		local, err = config.LoadLocalFile(localPath)
	} else if !os.IsNotExist(statErr) {
		err = statErr
	}
	if err != nil {
		return nil, fmt.Errorf("loading local config: %w", err)
	}
	return &Resolution{nil, global, local, globalPath, localPath, root}, nil
}

// Resolve combines trusted configuration with profiles without mutating inputs.
func Resolve(global, local *config.Config, root string) (*config.Config, error) {
	cfg := config.Merge(global, local)
	if err := ApplyProfiles(cfg, root, false); err != nil {
		return nil, err
	}
	if local != nil {
		cfg.Features = mergeLocalFeatures(cfg.Features, local.Features)
		if cfg.Env == nil {
			cfg.Env = map[string]string{}
		}
		for key, value := range local.Env {
			cfg.Env[key] = value
		}
	}
	ExpandWorkspace(cfg, root)
	return cfg, nil
}

func mergeLocalFeatures(resolved, local *config.FeaturesConfig) *config.FeaturesConfig {
	merged := config.MergeFeatures(resolved, local)
	if resolved == nil || merged == nil {
		return merged
	}
	no, yes := false, true
	restrict := func(source *bool, target **bool) {
		if source != nil && !*source {
			*target = &no
		}
	}
	restrict(resolved.EnableSSH, &merged.EnableSSH)
	restrict(resolved.EnableX11, &merged.EnableX11)
	restrict(resolved.EnableDBus, &merged.EnableDBus)
	restrict(resolved.EnableProxy, &merged.EnableProxy)
	restrict(resolved.AllowRawDBus, &merged.AllowRawDBus)
	restrict(resolved.AutoRepoDeployKey, &merged.AutoRepoDeployKey)
	restrict(resolved.EnableWSL, &merged.EnableWSL)
	for _, pair := range []struct {
		source *bool
		target **bool
	}{
		{resolved.NoNet, &merged.NoNet}, {resolved.UnshareNet, &merged.UnshareNet},
		{resolved.MaskHistory, &merged.MaskHistory}, {resolved.BlockGH, &merged.BlockGH},
	} {
		if pair.source != nil && *pair.source {
			*pair.target = &yes
		}
	}
	return merged
}
