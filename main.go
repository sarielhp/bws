package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"bw/internal/bwrap"
	"bw/internal/cli"
	"bw/internal/config"
	"bw/internal/sandbox"
	"bw/internal/util"

	"github.com/sarielhp/clihelp"
)

var Version = "0.1.1"

func main() {
	var forceFlag bool
	var infoFlag bool

	app := &clihelp.App{
		Name:        "bw",
		Description: "Launch a secure, isolated bubblewrap sandbox with configurable bindings, SSH agent forwarding, X11 support, and shell theming.",
		Version:     Version,
		PersistentOptions: []clihelp.Option{
			clihelp.Bool(&forceFlag, "-f, --force", false, "Bypass the file count safety check"),
			clihelp.Bool(&infoFlag, "--info", false, "Print the sandbox configuration plan and exit (dry run; no side effects)"),
		},
		Commands: []clihelp.Command{
			{
				Name:        "scp",
				Description: "Copy the global config and theme to a remote host via scp",
				UsageLine:   "bw scp <user@host:>",
				Args:        clihelp.ExactArgs(1),
				Examples: []clihelp.Example{
					{Line: "bw scp user@host:", Description: "Copy config to home directory on remote host"},
					{Line: "bw scp user@host:/path/to/dir", Description: "Copy config to a specific remote directory"},
				},
				Run: func(ctx *clihelp.Context) error {
					cli.HandleSCP(ctx.Args)
					return nil
				},
			},
			{
				Name:        "copy",
				Description: "Manage the list of programs copied into the sandbox from the host",
				UsageLine:   "bw copy <subcommand> [args...]",
				Subcommands: []clihelp.Command{
					{
						Name:        "add",
						Description: "Add an absolute path to the global copy list",
						UsageLine:   "bw copy add <absolute-path>",
						Args:        clihelp.ExactArgs(1),
						Examples: []clihelp.Example{
							{Line: "bw copy add /home/user/bin/myprog"},
						},
						Run: func(ctx *clihelp.Context) error {
							cli.HandleCopyAdd(ctx.Args[0])
							return nil
						},
					},
					{
						Name:        "list",
						Aliases:     []string{"ls"},
						Description: "List all programs configured in the global copy list",
						UsageLine:   "bw copy list",
						Args:        clihelp.NoArgs,
						Run: func(ctx *clihelp.Context) error {
							cli.HandleCopyList()
							return nil
						},
					},
					{
						Name:        "del",
						Aliases:     []string{"delete", "rm", "remove"},
						Description: "Remove an absolute path from the global copy list",
						UsageLine:   "bw copy del <absolute-path>",
						Args:        clihelp.ExactArgs(1),
						Examples: []clihelp.Example{
							{Line: "bw copy del /home/user/bin/myprog"},
						},
						Run: func(ctx *clihelp.Context) error {
							cli.HandleCopyDel(ctx.Args[0])
							return nil
						},
					},
				},
			},
			{
				Name:        "test",
				Description: "Run a verification of a tool inside the sandbox and exit",
				UsageLine:   "bw test <target>",
				Subcommands: []clihelp.Command{
					{
						Name:        "opencode",
						Description: "Verify opencode loads correctly inside the sandbox",
						UsageLine:   "bw test opencode",
						Args:        clihelp.NoArgs,
						Run: func(ctx *clihelp.Context) error {
							return runSandboxCommand("opencode", []string{"opencode", "debug", "info"}, forceFlag)
						},
					},
					{
						Name:        "quarto",
						Description: "Verify quarto loads correctly inside the sandbox",
						UsageLine:   "bw test quarto",
						Args:        clihelp.NoArgs,
						Run: func(ctx *clihelp.Context) error {
							return runSandboxCommand("quarto", []string{"quarto", "--version"}, forceFlag)
						},
					},
					{
						Name:        "uv",
						Description: "Verify uv and uvx load correctly inside the sandbox",
						UsageLine:   "bw test uv",
						Args:        clihelp.NoArgs,
						Run: func(ctx *clihelp.Context) error {
							return runUVTest(forceFlag)
						},
					},
				},
			},
		},
		Run: func(ctx *clihelp.Context) error {
			return runDefault(ctx.Args, forceFlag, infoFlag)
		},
	}

	if err := app.Execute(os.Args[1:]); err != nil {
		clihelp.PrintError(err)
		os.Exit(1)
	}
}

type sandboxLaunch struct {
	cfg        *config.Config
	globalCfg  *config.Config
	localCfg   *config.Config
	globalPath string
	localPath  string
}

func loadConfigs() (*sandboxLaunch, error) {
	globalPath := config.GlobalPath()
	globalCfg, err := config.LoadFile(globalPath)
	if err != nil {
		if os.IsNotExist(err) {
			config.CreateDefault(globalPath)
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
		localCfg, err = config.LoadFile(localPath)
		if err != nil {
			return nil, fmt.Errorf("loading local config: %w", err)
		}
	}

	mergedCfg := config.Merge(globalCfg, localCfg)

	return &sandboxLaunch{
		cfg:        mergedCfg,
		globalCfg:  globalCfg,
		localCfg:   localCfg,
		globalPath: globalPath,
		localPath:  localPath,
	}, nil
}

func safetyChecks(sl *sandboxLaunch, force bool) (string, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting current directory: %w", err)
	}

	homeDir := util.HomeDir()
	homeDirReal, _ := filepath.EvalSymlinks(homeDir)
	currentDirReal, _ := filepath.EvalSymlinks(currentDir)

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
		if count > fileLimit {
			return "", fmt.Errorf("current directory contains more than %d files (found %d); use -f to override", fileLimit, count)
		}
	}

	return currentDir, nil
}

func buildAndRun(sl *sandboxLaunch, currentDir string, dryRun bool, execArgs []string) error {
	sandboxDir := sl.cfg.SandboxPath
	if sandboxDir == "" {
		sandboxDir = filepath.Join(util.HomeDir(), ".sandbox", "pi_generic")
	}
	sandboxDir = util.ExpandHome(sandboxDir)

	if !dryRun {
		sandbox.Prepare(sl.cfg, sandboxDir)
	}

	bwrapArgs := bwrap.BuildArgs(sl.cfg, sandboxDir, currentDir, dryRun)

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

func runDefault(args []string, force, info bool) error {
	sl, err := loadConfigs()
	if err != nil {
		return err
	}

	isDefaultSession := len(args) == 0 && !info
	cli.VerifyTools(isDefaultSession, info)

	if !info {
		cli.VerifyBwrapUserns()
	}

	currentDir, err := safetyChecks(sl, force)
	if err != nil {
		return err
	}

	if info {
		return buildAndRun(sl, currentDir, true, nil)
	}

	var execArgs []string
	if len(args) == 0 {
		sessionName := "bwrap-dev"
		if sl.cfg.TmuxSessionName != "" {
			sessionName = sl.cfg.TmuxSessionName
		}
		execArgs = []string{"tmux", "-u", "new-session", "-A", "-s", sessionName}
	} else {
		execArgs = args
	}

	return buildAndRun(sl, currentDir, false, execArgs)
}

func runSandboxCommand(name string, execArgs []string, force bool) error {
	sl, err := loadConfigs()
	if err != nil {
		return err
	}

	cli.VerifyTools(false, false)
	cli.VerifyBwrapUserns()

	currentDir, err := safetyChecks(sl, force)
	if err != nil {
		return err
	}

	sandboxDir := sl.cfg.SandboxPath
	if sandboxDir == "" {
		sandboxDir = filepath.Join(util.HomeDir(), ".sandbox", "pi_generic")
	}
	sandboxDir = util.ExpandHome(sandboxDir)

	sandbox.Prepare(sl.cfg, sandboxDir)
	bwrapArgs := bwrap.BuildArgs(sl.cfg, sandboxDir, currentDir, false)

	cmd := exec.Command("bwrap", append(bwrapArgs, execArgs...)...)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("test failed: %s did not load correctly:\n%s", name, string(output))
	}

	switch name {
	case "quarto":
		fmt.Printf("Quarto version inside sandbox: %s", string(output))
	case "uv":
		fmt.Printf("uv version inside sandbox: %s", string(output))
	}
	fmt.Println("Everything is fine.")
	return nil
}

func runUVTest(force bool) error {
	sl, err := loadConfigs()
	if err != nil {
		return err
	}

	cli.VerifyTools(false, false)
	cli.VerifyBwrapUserns()

	currentDir, err := safetyChecks(sl, force)
	if err != nil {
		return err
	}

	sandboxDir := sl.cfg.SandboxPath
	if sandboxDir == "" {
		sandboxDir = filepath.Join(util.HomeDir(), ".sandbox", "pi_generic")
	}
	sandboxDir = util.ExpandHome(sandboxDir)

	sandbox.Prepare(sl.cfg, sandboxDir)
	bwrapArgs := bwrap.BuildArgs(sl.cfg, sandboxDir, currentDir, false)

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
