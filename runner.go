package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"bws/internal/bwrap"
	"bws/internal/cli"
	"bws/internal/config"
	"bws/internal/dbus"
	"bws/internal/proxy"
	"bws/internal/sandbox"
	"bws/internal/util"
)

func setupProxyService(cfg *config.Config, dryRun, verbose bool) (*proxy.Server, error) {
	if dryRun || !config.FeatureEnabledDefault(cfg, func(f *config.FeaturesConfig) *bool { return f.EnableProxy }, false) {
		return nil, nil
	}
	proxyServer, err := proxy.Start()
	if err != nil {
		return nil, fmt.Errorf("starting internal proxy: %w", err)
	}
	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] Started ephemeral proxy: %s\n", proxyServer.URL())
	}
	if cfg.Env == nil {
		cfg.Env = make(map[string]string)
	}
	proxyURL := proxyServer.URL()
	cfg.Env["HTTP_PROXY"] = proxyURL
	cfg.Env["HTTPS_PROXY"] = proxyURL
	cfg.Env["http_proxy"] = proxyURL
	cfg.Env["https_proxy"] = proxyURL
	cfg.Env["NO_PROXY"] = "localhost,127.0.0.1"
	cfg.Env["no_proxy"] = "localhost,127.0.0.1"
	return proxyServer, nil
}

func setupDBusService(cfg *config.Config, dryRun, verbose bool) *dbus.Proxy {
	if dryRun || !config.FeatureEnabledDefault(cfg, func(f *config.FeaturesConfig) *bool { return f.EnableDBus }, false) {
		return nil
	}
	dbusProxy, err := dbus.Start(cfg, verbose)
	if err != nil && verbose {
		fmt.Fprintf(os.Stderr, "[verbose] Starting D-Bus proxy failed: %v\n", err)
	}
	return dbusProxy
}

func setupSandboxHome(cfg *config.Config, currentDir string, dryRun, verbose bool, getDBus func() *dbus.Proxy) (string, func(), error) {
	if cfg.SandboxPath != "" {
		sandboxDir := util.ExpandHome(cfg.SandboxPath)
		if verbose {
			fmt.Fprintf(os.Stderr, "[verbose] Using configured sandbox directory: %s\n", sandboxDir)
		}
		if !dryRun {
			sandbox.Prepare(cfg, sandboxDir)
		}
		return sandboxDir, nil, nil
	}
	if dryRun {
		return "<ephemeral staged home>", nil, nil
	}
	sandboxDir, cleanup, err := sandbox.StageHome(cfg, currentDir)
	if err != nil {
		return "", nil, err
	}
	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] Staged ephemeral sandbox home: %s\n", sandboxDir)
	}
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-sigChan
		if p := getDBus(); p != nil {
			_ = p.Close()
		}
		cleanup()
		os.Exit(130)
	}()
	return sandboxDir, cleanup, nil
}

func buildAndRun(sl *sandboxLaunch, currentDir string, dryRun bool, execArgs []string, verbose bool) error {
	if !dryRun {
		if err := util.EnsureBwrap(); err != nil {
			return err
		}
	}

	var dbusProxy *dbus.Proxy
	sandboxDir, cleanup, err := setupSandboxHome(sl.cfg, currentDir, dryRun, verbose, func() *dbus.Proxy { return dbusProxy })
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] Current directory: %s\n", currentDir)
	}

	proxyServer, err := setupProxyService(sl.cfg, dryRun, verbose)
	if err != nil {
		return err
	}
	if proxyServer != nil {
		defer proxyServer.Close()
	}

	dbusProxy = setupDBusService(sl.cfg, dryRun, verbose)
	if dbusProxy != nil {
		defer dbusProxy.Close()
	}

	bwrapArgs := bwrap.BuildArgs(sl.cfg, sandboxDir, currentDir, dryRun, verbose)
	if dbusProxy != nil && !dbusProxy.IsRaw() && dbusProxy.SocketPath() != "" {
		bwrapArgs = append(bwrapArgs,
			"--bind", dbusProxy.SocketPath(), dbusProxy.DestPath(),
			"--setenv", "DBUS_SESSION_BUS_ADDRESS", fmt.Sprintf("unix:path=%s", dbusProxy.DestPath()),
			"--setenv", "XDG_RUNTIME_DIR", dbusProxy.DestDir(),
		)
	}

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
	runErr := cmd.Run()
	if dbusProxy != nil {
		_ = dbusProxy.Close()
	}
	if cleanup != nil {
		cleanup()
	}
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return runErr
	}
	return nil
}

func runDefault(args []string, force, verbose, noSSH, noNet, proxy, noProxy, dbusFlag, noDBus, noInit bool) error {
	sl, err := loadConfigs(verbose)
	if err != nil {
		return err
	}

	applyFlags(sl.cfg, noSSH, noNet, proxy, noProxy, dbusFlag, noDBus)

	currentDir, err := safetyChecks(sl, force, verbose)
	if err != nil {
		return err
	}

	if err := maybeAutoInit(sl, currentDir, force, noInit, noSSH, noNet, proxy, noProxy, dbusFlag, noDBus, verbose); err != nil {
		return err
	}

	isDefaultSession := len(args) == 0
	cli.VerifyTools(isDefaultSession, false)
	cli.VerifyBwrapUserns()

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

func runStatus(showAll, verbose, noSSH, noNet, proxy, noProxy, dbusFlag, noDBus bool) error {
	if showAll {
		return runConf(verbose, noSSH, noNet, proxy, noProxy, dbusFlag, noDBus)
	}
	return cli.HandleStatusShort()
}

func runConf(verbose, noSSH, noNet, proxy, noProxy, dbusFlag, noDBus bool) error {
	sl, err := loadConfigs(verbose)
	if err != nil {
		return err
	}

	applyFlags(sl.cfg, noSSH, noNet, proxy, noProxy, dbusFlag, noDBus)

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
	var dbusProxy *dbus.Proxy

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

	if config.FeatureEnabledDefault(sl.cfg, func(f *config.FeaturesConfig) *bool { return f.EnableDBus }, false) {
		dbusProxy, _ = dbus.Start(sl.cfg, verbose)
		if dbusProxy != nil {
			defer dbusProxy.Close()
		}
	}

	bwrapArgs := bwrap.BuildArgs(sl.cfg, sandboxDir, currentDir, false, verbose)
	if dbusProxy != nil && !dbusProxy.IsRaw() && dbusProxy.SocketPath() != "" {
		bwrapArgs = append(bwrapArgs,
			"--bind", dbusProxy.SocketPath(), dbusProxy.DestPath(),
			"--setenv", "DBUS_SESSION_BUS_ADDRESS", fmt.Sprintf("unix:path=%s", dbusProxy.DestPath()),
			"--setenv", "XDG_RUNTIME_DIR", dbusProxy.DestDir(),
		)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] bwrap command:\n  bwrap %s\n", strings.Join(append(bwrapArgs, execArgs...), " "))
	}

	cmd := exec.Command("bwrap", append(bwrapArgs, execArgs...)...)
	output, err := cmd.Output()
	if dbusProxy != nil {
		_ = dbusProxy.Close()
	}
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

func runExec(args []string, force, verbose, noSSH, noNet, proxy, noProxy, dbusFlag, noDBus bool) error {
	sl, err := loadConfigs(verbose)
	if err != nil {
		return err
	}

	applyFlags(sl.cfg, noSSH, noNet, proxy, noProxy, dbusFlag, noDBus)

	cli.VerifyTools(false, false)
	cli.VerifyBwrapUserns()

	currentDir, err := safetyChecks(sl, force, verbose)
	if err != nil {
		return err
	}

	return buildAndRun(sl, currentDir, false, args, verbose)
}
