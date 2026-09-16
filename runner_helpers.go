package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bws/internal/cli"
	"bws/internal/config"
	"bws/internal/profile"
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
			globalCfg, err = config.Parse([]byte(config.DefaultConfigTemplate), globalPath)
			if err != nil {
				return nil, err
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
		localCfg, err = config.LoadLocalFile(localPath)
		if err != nil {
			return nil, fmt.Errorf("loading local config: %w", err)
		}
	} else if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] No local config found at: %s\n", localPath)
	}

	mergedCfg := config.Merge(globalCfg, localCfg)
	currentDir, _ := os.Getwd()
	if err := applyProfiles(mergedCfg, currentDir, verbose); err != nil {
		return nil, fmt.Errorf("applying profiles: %w", err)
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

func mergeResolvedProfile(cfg *config.Config, resolved *profile.ResolvedProfile, seenPassEnv, seenMask, seenCopy, seenRW, seenRO, seenPath map[string]bool) {
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
	for _, c := range resolved.Copy {
		if !seenCopy[c] {
			seenCopy[c] = true
			cfg.Copy = append(cfg.Copy, c)
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
	if resolved.Features != nil {
		cfg.Features = config.MergeFeatures(cfg.Features, resolved.Features)
	}
	if resolved.UnshareNet {
		if cfg.Features == nil {
			cfg.Features = &config.FeaturesConfig{}
		}
		t := true
		cfg.Features.NoNet = &t
	}
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
	seenCopy := make(map[string]bool)
	for _, c := range cfg.Copy {
		seenCopy[c] = true
	}

	var allResolved []string
	seenResolved := make(map[string]bool)

	for _, pName := range cfg.Profiles {
		resolved, err := profile.ResolveProfile(pName, registry, ctx)
		if err != nil {
			return fmt.Errorf("resolving profile %q: %w", pName, err)
		}
		for _, rName := range resolved.Profiles {
			if !seenResolved[rName] {
				seenResolved[rName] = true
				allResolved = append(allResolved, rName)
			}
		}
		mergeResolvedProfile(cfg, resolved, seenPassEnv, seenMask, seenCopy, seenRW, seenRO, seenPath)
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

	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] Current directory: %s\n", currentDir)
	}
	if err := config.ValidateWorkspace(currentDir, sl.cfg.MaxFileCount, force); err != nil {
		return "", err
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
	if noProxy {
		f := false
		cfg.Features.EnableProxy = &f
	} else if proxy {
		t := true
		cfg.Features.EnableProxy = &t
	}
	if noDBus {
		f := false
		cfg.Features.EnableDBus = &f
	} else if dbus {
		t := true
		cfg.Features.EnableDBus = &t
	}
}

func maybeAutoInit(sl *sandboxLaunch, currentDir string, force, noInit, noSSH, noNet, proxy, noProxy, dbusFlag, noDBus, verbose bool) error {
	if sl.localCfg != nil {
		return nil
	}
	localConfigPath := filepath.Join(currentDir, ".bws", "config.jsonc")
	if _, err := os.Stat(localConfigPath); err == nil {
		return nil
	}

	if force || noInit {
		return nil
	}

	mode := config.AutoInitMode(sl.cfg)
	if mode == "never" {
		return nil
	}

	if mode == "prompt" {
		if !cli.IsInteractiveTTY(int(os.Stdin.Fd())) {
			return nil
		}
		accepted, err := cli.PromptAutoInit(os.Stdin, os.Stderr)
		if err != nil || !accepted {
			return err
		}
	}

	configPath, summary, err := cli.AutoConfigureWorkspace(currentDir, noSSH)
	if err != nil {
		return fmt.Errorf("auto-configuring workspace: %w", err)
	}
	if summary == "" {
		return nil
	}

	fmt.Fprintf(os.Stderr, "[bws] Auto-configured .bws/config.jsonc (%s detected)\n", summary)

	localCfg, err := config.LoadLocalFile(configPath)
	if err != nil {
		return fmt.Errorf("loading auto-configured local config: %w", err)
	}

	sl.localCfg = localCfg
	sl.localPath = configPath
	sl.cfg = config.Merge(sl.globalCfg, localCfg)
	if err := applyProfiles(sl.cfg, currentDir, verbose); err != nil {
		return fmt.Errorf("applying profiles: %w", err)
	}
	applyFlags(sl.cfg, noSSH, noNet, proxy, noProxy, dbusFlag, noDBus)
	return nil
}
