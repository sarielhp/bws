package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bws/internal/config"
	"bws/internal/profile"
	"bws/internal/util"
)

type sandboxLaunch struct {
	cfg        *config.Config
	globalCfg  *config.Config
	localCfg   *config.Config
	globalPath string
	localPath  string
}

func loadConfigs(verbose bool) (*sandboxLaunch, error) {
	globalPath := config.GlobalPath()
	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] Loading global config: %s\n", globalPath)
	}
	globalCfg, err := config.LoadFile(globalPath)
	if err != nil {
		if os.IsNotExist(err) {
			_ = config.CreateDefault(globalPath)
			examplePath := filepath.Join(filepath.Dir(globalPath), "example-config.jsonc")
			_ = config.CreateExampleConfig(examplePath)
			fmt.Printf("Created config file: %s\n", globalPath)
			globalCfg, _ = config.LoadFile(globalPath)
			if globalCfg == nil {
				globalCfg = &config.Config{}
			}
		} else {
			return nil, fmt.Errorf("loading global config: %w", err)
		}
	}

	localPath := config.LocalPath()
	var localCfg *config.Config
	if fi, err := os.Stat(localPath); err == nil && !fi.IsDir() {
		if verbose {
			fmt.Fprintf(os.Stderr, "[verbose] Loading local config: %s\n", localPath)
		}
		localCfg, err = config.LoadFile(localPath)
		if err != nil {
			return nil, fmt.Errorf("loading local config: %w", err)
		}
	} else if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] No local config found at: %s\n", localPath)
	}

	mergedCfg := config.Merge(globalCfg, localCfg)
	currentDir, _ := os.Getwd()
	if err := applyProfiles(mergedCfg, currentDir, verbose); err != nil && verbose {
		fmt.Fprintf(os.Stderr, "[verbose] Applying profiles warning: %v\n", err)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] Merged config PATH entries: %s\n", strings.Join(mergedCfg.Path, ", "))
	}

	return &sandboxLaunch{
		cfg:        mergedCfg,
		globalCfg:  globalCfg,
		localCfg:   localCfg,
		globalPath: globalPath,
		localPath:  localPath,
	}, nil
}

func applyProfiles(cfg *config.Config, currentDir string, verbose bool) error {
	if len(cfg.Profiles) == 0 {
		return nil
	}
	registry, err := profile.LoadRegistry(currentDir)
	if err != nil {
		return err
	}
	ctx := profile.DetectMatchContext()

	seenRW := make(map[string]bool)
	for _, b := range cfg.BindsRW {
		seenRW[b.Host+"->"+b.Sandbox] = true
	}
	seenRO := make(map[string]bool)
	for _, b := range cfg.BindsRO {
		seenRO[b.Host+"->"+b.Sandbox] = true
	}
	seenPath := make(map[string]bool)
	for _, p := range cfg.Path {
		seenPath[p] = true
	}
	seenPassEnv := make(map[string]bool)
	for _, pe := range cfg.PassEnv {
		seenPassEnv[pe] = true
	}
	seenMask := make(map[string]bool)
	for _, m := range cfg.Mask {
		seenMask[m] = true
	}

	var allResolved []string
	seenResolved := make(map[string]bool)

	for _, pName := range cfg.Profiles {
		resolved, err := profile.ResolveProfile(pName, registry, ctx)
		if err != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "[verbose] Warning: resolving profile %q: %v\n", pName, err)
			}
			continue
		}
		for _, rName := range resolved.Profiles {
			if !seenResolved[rName] {
				seenResolved[rName] = true
				allResolved = append(allResolved, rName)
			}
		}
		for _, pe := range resolved.PassEnv {
			if !seenPassEnv[pe] {
				seenPassEnv[pe] = true
				cfg.PassEnv = append(cfg.PassEnv, pe)
			}
		}
		for _, m := range resolved.Mask {
			if !seenMask[m] {
				seenMask[m] = true
				cfg.Mask = append(cfg.Mask, m)
			}
		}
		for _, b := range resolved.BindsRW {
			key := b[0] + "->" + b[1]
			if !seenRW[key] {
				seenRW[key] = true
				cfg.BindsRW = append(cfg.BindsRW, config.BindEntry{Host: b[0], Sandbox: b[1]})
			}
		}
		for _, b := range resolved.BindsRO {
			key := b[0] + "->" + b[1]
			if !seenRO[key] {
				seenRO[key] = true
				cfg.BindsRO = append(cfg.BindsRO, config.BindEntry{Host: b[0], Sandbox: b[1]})
			}
		}
		for _, pt := range resolved.Path {
			if !seenPath[pt] {
				seenPath[pt] = true
				cfg.Path = append(cfg.Path, pt)
			}
		}
		for k, v := range resolved.Env {
			if cfg.Env == nil {
				cfg.Env = make(map[string]string)
			}
			if _, exists := cfg.Env[k]; !exists {
				cfg.Env[k] = v
			}
		}
		if resolved.UnshareNet {
			if cfg.Features == nil {
				cfg.Features = &config.FeaturesConfig{}
			}
			t := true
			cfg.Features.NoNet = &t
		}
	}
	if len(allResolved) > 0 {
		if cfg.Env == nil {
			cfg.Env = make(map[string]string)
		}
		cfg.Env["BWS_ACTIVE_PROFILES"] = strings.Join(allResolved, ",")
	}
	return nil
}

func safetyChecks(sl *sandboxLaunch, force, verbose bool) (string, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting current directory: %w", err)
	}

	homeDir := util.HomeDir()
	homeDirReal, _ := filepath.EvalSymlinks(homeDir)
	currentDirReal, _ := filepath.EvalSymlinks(currentDir)

	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] Current directory: %s\n", currentDir)
		fmt.Fprintf(os.Stderr, "[verbose] Home directory: %s\n", homeDir)
	}

	if currentDirReal == "/" {
		return "", fmt.Errorf("running the sandbox from / is blocked")
	}
	if currentDirReal == homeDirReal {
		return "", fmt.Errorf("running the sandbox from ~/ is blocked")
	}
	homeBinDir := filepath.Join(homeDirReal, "bin")
	if currentDirReal == homeBinDir {
		return "", fmt.Errorf("running the sandbox from ~/bin/ is blocked")
	}

	fileLimit := 1000
	if sl.cfg.MaxFileCount > 0 {
		fileLimit = sl.cfg.MaxFileCount
	}
	if !force {
		count := util.CountFiles(currentDir, fileLimit)
		if verbose {
			fmt.Fprintf(os.Stderr, "[verbose] File count: %d (limit: %d)\n", count, fileLimit)
		}
		if count > fileLimit {
			return "", fmt.Errorf("current directory contains more than %d files (found %d); use -f to override", fileLimit, count)
		}
	} else if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] File count check bypassed (-f)\n")
	}

	return currentDir, nil
}

func applyFlags(cfg *config.Config, noSSH, noNet, proxy, noProxy, dbus, noDBus bool) {
	if cfg == nil {
		return
	}
	if cfg.Features == nil {
		cfg.Features = &config.FeaturesConfig{}
	}
	if noSSH {
		f := false
		cfg.Features.EnableSSH = &f
	}
	if noNet {
		t := true
		cfg.Features.NoNet = &t
	}
	if proxy {
		t := true
		cfg.Features.EnableProxy = &t
	} else if noProxy {
		f := false
		cfg.Features.EnableProxy = &f
	}
	if dbus {
		t := true
		cfg.Features.EnableDBus = &t
	} else if noDBus {
		f := false
		cfg.Features.EnableDBus = &f
	}
}
