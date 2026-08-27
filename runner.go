package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"bws/internal/bwrap"
	"bws/internal/cli"
	"bws/internal/config"
	"bws/internal/profile"
	"bws/internal/sandbox"
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
			config.CreateDefault(globalPath)
			examplePath := filepath.Join(filepath.Dir(globalPath), "example-config.jsonc")
			config.CreateExampleConfig(examplePath)
			themePath := filepath.Join(util.HomeDir(), ".config", "bw", "theme.omp.json")
			config.CreateDefaultTheme(themePath)
			fmt.Printf("Created config file: %s\n", globalPath)
			os.Exit(0)
		}
		return nil, fmt.Errorf("loading global config: %w", err)
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

	for _, pName := range cfg.Profiles {
		resolved, err := profile.ResolveProfile(pName, registry, ctx)
		if err != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "[verbose] Warning: resolving profile %q: %v\n", pName, err)
			}
			continue
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

func buildAndRun(sl *sandboxLaunch, currentDir string, dryRun bool, execArgs []string, verbose bool) error {
	var sandboxDir string
	var cleanup func()

	if sl.cfg.SandboxPath != "" {
		sandboxDir = util.ExpandHome(sl.cfg.SandboxPath)
		if verbose {
			fmt.Fprintf(os.Stderr, "[verbose] Using configured sandbox directory: %s\n", sandboxDir)
		}
		if !dryRun {
			sandbox.Prepare(sl.cfg, sandboxDir)
		}
	} else if !dryRun {
		var err error
		sandboxDir, cleanup, err = sandbox.StageHome(sl.cfg, currentDir)
		if err != nil {
			return err
		}
		defer cleanup()
		if verbose {
			fmt.Fprintf(os.Stderr, "[verbose] Staged ephemeral sandbox home: %s\n", sandboxDir)
		}
	} else {
		sandboxDir = "<ephemeral staged home>"
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] Current directory: %s\n", currentDir)
	}

	bwrapArgs := bwrap.BuildArgs(sl.cfg, sandboxDir, currentDir, dryRun, verbose)

	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] bwrap command:\n  bwrap %s\n", strings.Join(append(bwrapArgs, execArgs...), " "))
	}

	if dryRun {
		cli.PrintInfo(bwrapArgs, sl.cfg, sl.globalPath, sl.localPath, currentDir)
		return nil
	}

	cmd := exec.Command("bwrap", append(bwrapArgs, execArgs...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return err
	}
	return nil
}

func runDefault(args []string, force, verbose bool) error {
	sl, err := loadConfigs(verbose)
	if err != nil {
		return err
	}

	isDefaultSession := len(args) == 0
	cli.VerifyTools(isDefaultSession, false)
	cli.VerifyBwrapUserns()

	currentDir, err := safetyChecks(sl, force, verbose)
	if err != nil {
		return err
	}

	var execArgs []string
	if len(args) == 0 {
		sessionName := "bwrap-dev"
		if sl.cfg.TmuxSessionName != "" {
			sessionName = sl.cfg.TmuxSessionName
		}
		// Use login shell to ensure profile is sourced
		execArgs = []string{"tmux", "-u", "new-session", "-A", "-s", sessionName, "/bin/bash", "-l"}
	} else {
		execArgs = args
	}

	return buildAndRun(sl, currentDir, false, execArgs, verbose)
}

func runConf(verbose bool) error {
	sl, err := loadConfigs(verbose)
	if err != nil {
		return err
	}

	currentDir, err := os.Getwd()
	if err != nil {
		return err
	}

	sandboxDir := sl.cfg.SandboxPath
	if sandboxDir == "" {
		sandboxDir = "<ephemeral staged home>"
	} else {
		sandboxDir = util.ExpandHome(sandboxDir)
	}

	bwrapArgs := bwrap.BuildArgs(sl.cfg, sandboxDir, currentDir, true, verbose)
	cli.PrintInfo(bwrapArgs, sl.cfg, sl.globalPath, sl.localPath, currentDir)
	return nil
}

func runSandboxCommand(name string, execArgs []string, force, verbose bool) error {
	sl, err := loadConfigs(verbose)
	if err != nil {
		return err
	}

	cli.VerifyTools(false, false)
	cli.VerifyBwrapUserns()

	currentDir, err := safetyChecks(sl, force, verbose)
	if err != nil {
		return err
	}

	var sandboxDir string
	var cleanup func()
	if sl.cfg.SandboxPath != "" {
		sandboxDir = util.ExpandHome(sl.cfg.SandboxPath)
		sandbox.Prepare(sl.cfg, sandboxDir)
	} else {
		var err error
		sandboxDir, cleanup, err = sandbox.StageHome(sl.cfg, currentDir)
		if err != nil {
			return err
		}
		defer cleanup()
	}

	bwrapArgs := bwrap.BuildArgs(sl.cfg, sandboxDir, currentDir, false, verbose)

	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] bwrap command:\n  bwrap %s\n", strings.Join(append(bwrapArgs, execArgs...), " "))
	}

	cmd := exec.Command("bwrap", append(bwrapArgs, execArgs...)...)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("test failed: %s did not load correctly:\n%s", name, string(output))
	}

	switch name {
	case "opencode":
		fmt.Printf("OpenCode version inside sandbox: %s", string(output))
	case "quarto":
		fmt.Printf("Quarto version inside sandbox: %s", string(output))
	case "uv":
		fmt.Printf("uv version inside sandbox: %s", string(output))
	}
	fmt.Println("Everything is fine.")
	return nil
}

func runUVTest(force, verbose bool) error {
	sl, err := loadConfigs(verbose)
	if err != nil {
		return err
	}

	cli.VerifyTools(false, false)
	cli.VerifyBwrapUserns()

	currentDir, err := safetyChecks(sl, force, verbose)
	if err != nil {
		return err
	}

	var sandboxDir string
	var cleanup func()
	if sl.cfg.SandboxPath != "" {
		sandboxDir = util.ExpandHome(sl.cfg.SandboxPath)
		sandbox.Prepare(sl.cfg, sandboxDir)
	} else {
		var err error
		sandboxDir, cleanup, err = sandbox.StageHome(sl.cfg, currentDir)
		if err != nil {
			return err
		}
		defer cleanup()
	}

	bwrapArgs := bwrap.BuildArgs(sl.cfg, sandboxDir, currentDir, false, verbose)

	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] bwrap command:\n  bwrap %s\n", strings.Join(append(bwrapArgs, "uv", "--version"), " "))
	}

	uvOut, uvErr := exec.Command("bwrap", append(bwrapArgs, "uv", "--version")...).Output()
	uvxOut, uvxErr := exec.Command("bwrap", append(bwrapArgs, "uvx", "--version")...).Output()

	if uvErr != nil || uvxErr != nil {
		if uvErr != nil {
			fmt.Fprintf(os.Stderr, "uv output: %s\n", string(uvOut))
		}
		if uvxErr != nil {
			fmt.Fprintf(os.Stderr, "uvx output: %s\n", string(uvxOut))
		}
		return fmt.Errorf("test failed: uv or uvx did not load correctly")
	}

	fmt.Printf("uv version inside sandbox: %s", string(uvOut))
	fmt.Printf("uvx version inside sandbox: %s", string(uvxOut))
	fmt.Println("Everything is fine.")
	return nil
}

func runExec(args []string, force, verbose bool) error {
	sl, err := loadConfigs(verbose)
	if err != nil {
		return err
	}

	cli.VerifyTools(false, false)
	cli.VerifyBwrapUserns()

	currentDir, err := safetyChecks(sl, force, verbose)
	if err != nil {
		return err
	}

	return buildAndRun(sl, currentDir, false, args, verbose)
}
