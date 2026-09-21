package main

import (
	"fmt"
	"os"
	"path/filepath"

	"bws/internal/cli"
	"bws/internal/config"
	"bws/internal/policy"
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
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	resolved, err := policy.Load(dir)
	if err != nil {
		return nil, err
	}
	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] Resolved configuration: %s + %s\n", resolved.GlobalPath, resolved.LocalPath)
	}
	return &sandboxLaunch{cfg: resolved.Config, globalCfg: resolved.Global, localCfg: resolved.Local, globalPath: resolved.GlobalPath, localPath: resolved.LocalPath}, nil
}

func applyProfiles(cfg *config.Config, currentDir string, verbose bool) error {
	return policy.ApplyProfiles(cfg, currentDir, verbose)
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
	policy.Flags{NoSSH: noSSH, NoNet: noNet, Proxy: proxy, NoProxy: noProxy, DBus: dbus, NoDBus: noDBus}.Apply(cfg)
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
		if err := cli.HandleInitOptions(cli.InitOptions{TargetDir: currentDir, Flags: policy.Flags{NoSSH: noSSH, NoNet: noNet, Proxy: proxy, NoProxy: noProxy, DBus: dbusFlag, NoDBus: noDBus}}); err != nil {
			return err
		}
		resolved, err := loadConfigs(verbose)
		if err != nil {
			return err
		}
		*sl = *resolved
		applyFlags(sl.cfg, noSSH, noNet, proxy, noProxy, dbusFlag, noDBus)
		return nil
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
	sl.cfg, err = policy.Resolve(sl.globalCfg, localCfg, currentDir)
	if err != nil {
		return fmt.Errorf("applying profiles: %w", err)
	}
	applyFlags(sl.cfg, noSSH, noNet, proxy, noProxy, dbusFlag, noDBus)
	return nil
}

func isShellName(name string) bool {
	base := filepath.Base(name)
	switch base {
	case "fish", "zsh", "bash", "sh", "csh", "tcsh", "dash":
		return true
	default:
		return false
	}
}

func resolveInteractiveShell(cfg *config.Config) []string {
	if cfg != nil {
		for _, p := range cfg.Profiles {
			switch p {
			case "fish":
				if util.CommandExists("fish") {
					return []string{"fish", "-l"}
				}
			case "zsh":
				if util.CommandExists("zsh") {
					return []string{"zsh", "-l"}
				}
			}
		}
	}

	if hostShell := os.Getenv("SHELL"); hostShell != "" {
		base := filepath.Base(hostShell)
		if isShellName(base) && util.CommandExists(base) {
			return []string{base, "-l"}
		}
	}

	if util.CommandExists("bash") {
		return []string{"/bin/bash", "-l"}
	}
	return []string{"/bin/sh", "-l"}
}
