package main

import (
	"fmt"
	"os"

	"bw/internal/cli"

	"github.com/sarielhp/clihelp"
)

var Version = "0.1.6"

func main() {
	var forceFlag bool
	var globalFlag bool
	var localFlag bool
	var roFlag bool

	app := &clihelp.App{
		Name:        "bw",
		Description: "Launch a secure bubblewrap sandbox with configurable bind mounts, SSH forwarding, X11, and shell theming.",
		Version:     Version,
		PersistentOptions: []clihelp.Option{
			clihelp.Bool(&forceFlag, "-f, --force", false, "Bypass the file count safety check"),
			clihelp.Bool(&globalFlag, "-g, --global", false, "Target the global config file (~/.config/bw/config.jsonc)"),
			clihelp.Bool(&localFlag, "-l, --local", false, "Target the local config file (.bw.jsonc in current directory)"),
		},
		Commands: []clihelp.Command{
			{
				Name:        "scp",
				Description: "Copy the global config and theme files to a remote host via scp",
				UsageLine:   "bw scp <user@host:>",
				Args:        clihelp.ExactArgs(1),
				Examples: []clihelp.Example{
					{Line: "bw scp user@host:", Description: "Copy config to home directory on remote host"},
				},
				Run: func(ctx *clihelp.Context) error {
					cli.HandleSCP(ctx.Args)
					return nil
				},
			},
			{
				Name:        "conf",
				Description: "Manage sandbox configuration files and view the merged config plan",
				UsageLine:   "bw conf [subcommand] [-g | -l]",
				Notes: []clihelp.Note{
					{Text: "Configuration is stored in two JSONC files: global (~/.config/bw/config.jsonc) and local (.bw.jsonc). The local config overrides the global for the current directory only. Without a subcommand, 'bw conf' shows the merged plan (same as the old --info flag). With -g or -l, it shows the raw file contents."},
				},
				Subcommands: []clihelp.Command{
					{
						Name:        "path",
						Description: "Print paths to both the global and local config files",
						UsageLine:   "bw conf path",
						Args:        clihelp.NoArgs,
						Run: func(ctx *clihelp.Context) error {
							cli.HandleConfigPath()
							return nil
						},
					},
					{
						Name:        "init",
						Description: "Regenerate a config file from default settings (backup old)",
						UsageLine:   "bw conf init -g | -l",
						Args:        clihelp.NoArgs,
						Run: func(ctx *clihelp.Context) error {
							cli.HandleConfigInit(globalFlag, localFlag)
							return nil
						},
					},
					{
						Name:        "edit",
						Description: "Open a config file in $EDITOR / $VISUAL / vi",
						UsageLine:   "bw conf edit -g | -l",
						Args:        clihelp.NoArgs,
						Run: func(ctx *clihelp.Context) error {
							cli.HandleConfigEdit(globalFlag, localFlag)
							return nil
						},
					},
					{
						Name:        "show",
						Description: "Display the raw contents of a config file",
						UsageLine:   "bw conf show -g | -l",
						Args:        clihelp.NoArgs,
						Run: func(ctx *clihelp.Context) error {
							if globalFlag {
								cli.HandleConfigShowGlobal()
							} else if localFlag {
								cli.HandleConfigShowLocal()
							} else {
								return fmt.Errorf("bw conf show requires -g or -l")
							}
							return nil
						},
					},
				},
				Run: func(ctx *clihelp.Context) error {
					if globalFlag || localFlag {
						if globalFlag {
							cli.HandleConfigShow(true)
						} else {
							cli.HandleConfigShow(false)
						}
						return nil
					}
					return runConf()
				},
			},
			{
				Name:        "ccopy",
				Description: "Manage files and programs copied into the sandbox",
				UsageLine:   "bw ccopy add|list|del [args...]",
				Notes: []clihelp.Note{
					{Text: "Copied files are snapshots of host files placed into the sandbox home before each launch. Unlike bind mounts, they are not live — changes on the host after the copy are not reflected in the sandbox. This is useful for tools and scripts that should be available without exposing the full host filesystem."},
				},
				Subcommands: []clihelp.Command{
					{
						Name:        "add",
						Description: "Add a program or file to the copy list (-g or -l required)",
						UsageLine:   "bw ccopy add <absolute-path> -g | -l",
						Args:        clihelp.ExactArgs(1),
						Examples: []clihelp.Example{
							{Line: "bw ccopy add /home/user/bin/myprog -g", Description: "Add globally"},
							{Line: "bw ccopy add /home/user/scripts/util.sh -l", Description: "Add locally"},
						},
						Run: func(ctx *clihelp.Context) error {
							cli.HandleCopyAdd(ctx.Args[0], globalFlag, localFlag)
							return nil
						},
					},
					{
						Name:        "list",
						Aliases:     []string{"ls"},
						Description: "List all configured copy paths from both configs",
						UsageLine:   "bw ccopy list",
						Args:        clihelp.NoArgs,
						Run: func(ctx *clihelp.Context) error {
							cli.HandleCopyList()
							return nil
						},
					},
					{
						Name:        "del",
						Aliases:     []string{"delete", "rm", "remove"},
						Description: "Remove a path from the copy list (-g or -l required)",
						UsageLine:   "bw ccopy del <absolute-path> -g | -l",
						Args:        clihelp.ExactArgs(1),
						Examples: []clihelp.Example{
							{Line: "bw ccopy del /home/user/bin/myprog -g", Description: "Remove globally"},
						},
						Run: func(ctx *clihelp.Context) error {
							cli.HandleCopyDel(ctx.Args[0], globalFlag, localFlag)
							return nil
						},
					},
				},
			},
			{
				Name:        "cbind",
				Description: "Manage bind mounts between the host and the sandbox",
				UsageLine:   "bw cbind add|list|del [args...]",
				Subcommands: []clihelp.Command{
					{
						Name:        "add",
						Description: "Add a bind mount (-g or -l required; --ro for read-only)",
						UsageLine:   "bw cbind add <host-path> [sandbox-path] -g | -l [--ro]",
						Args:        clihelp.RangeArgs(1, 2),
						Options: []clihelp.Option{
							clihelp.Bool(&roFlag, "--ro", false, "Make the bind mount read-only"),
						},
						Notes: []clihelp.Note{
							{Text: "A bind mount makes a host directory or file accessible inside the sandbox. Read-write (default): the sandbox can modify the host file. Read-only (--ro): the sandbox can only read. If sandbox-path is omitted, the host path is used as-is inside the sandbox."},
						},
						Examples: []clihelp.Example{
							{Line: "bw cbind add /home/user/projects /projects -g", Description: "RW global bind"},
							{Line: "bw cbind add /usr/share/dict --ro -l", Description: "RO local bind"},
						},
						Run: func(ctx *clihelp.Context) error {
							hostPath := ctx.Args[0]
							sandboxPath := ""
							if len(ctx.Args) > 1 {
								sandboxPath = ctx.Args[1]
							}
							cli.HandleBindAdd(hostPath, sandboxPath, roFlag, globalFlag, localFlag)
							return nil
						},
					},
					{
						Name:        "list",
						Aliases:     []string{"ls"},
						Description: "List all configured bind mounts from both configs",
						UsageLine:   "bw cbind list",
						Args:        clihelp.NoArgs,
						Run: func(ctx *clihelp.Context) error {
							cli.HandleBindList()
							return nil
						},
					},
					{
						Name:        "del",
						Aliases:     []string{"delete", "rm", "remove"},
						Description: "Remove a bind mount by host path (-g or -l required)",
						UsageLine:   "bw cbind del <host-path> -g | -l",
						Args:        clihelp.ExactArgs(1),
						Examples: []clihelp.Example{
							{Line: "bw cbind del /home/user/projects -g", Description: "Remove global bind"},
						},
						Run: func(ctx *clihelp.Context) error {
							cli.HandleBindDel(ctx.Args[0], globalFlag, localFlag)
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
			return runDefault(ctx.Args, forceFlag)
		},
	}

	if err := app.Execute(os.Args[1:]); err != nil {
		clihelp.PrintError(err)
		os.Exit(1)
	}
}
