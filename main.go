package main

import (
	"os"
	"strings"

	"bw/internal/cli"

	"github.com/sarielhp/clihelp"
)

var Version = "0.1.24"

func main() {
	var forceFlag bool
	var globalFlag bool
	var localFlag bool
	var roFlag bool
	var verboseFlag bool

	var dryRunFlag bool
	var noSSHFlag bool
	var opencodeFlag bool
	var presetFlag string
	var profileFlag string

	app := &clihelp.App{
		Name:        "bw",
		Description: "Launch a secure bubblewrap sandbox with configurable bind mounts, SSH forwarding, X11, and shell theming.",
		Version:     Version,
		PersistentOptions: []clihelp.Option{
			clihelp.Bool(&forceFlag, "-f, --force", false, "Bypass the file count safety check / force overwrite"),
			clihelp.Bool(&globalFlag, "-g, --global", false, "Target the global config file (~/.config/bw/config.jsonc)"),
			clihelp.Bool(&localFlag, "-l, --local", false, "Target the local config file (.bw.jsonc in current directory)"),
			clihelp.Bool(&verboseFlag, "-v, --verbose", false, "Print verbose debug information (config paths, bwrap args, etc.)"),
		},
		Commands: []clihelp.Command{
			{
				Name:        "init-dev",
				Description: "Automatically inspect workspace and create a hardened .bw.jsonc development configuration",
				UsageLine:   "bw init-dev [options] [target-dir]",
				Args:        clihelp.RangeArgs(0, 1),
				Options: []clihelp.Option{
					clihelp.Bool(&dryRunFlag, "-n, --dry-run", false, "Print generated configuration to stdout without writing to disk"),
					clihelp.Bool(&noSSHFlag, "--no-ssh", false, "Disable SSH forwarding and Git SSH commands"),
					clihelp.Bool(&opencodeFlag, "--opencode", false, "Force inclusion of OpenCode configuration directories"),
					clihelp.String(&presetFlag, "--preset", "", "Explicitly select a preset stack (go, python, rust, node, latex, agent, all)"),
					clihelp.String(&profileFlag, "-p, --profile", "", "Explicitly include tool profile(s), comma-separated (e.g. -p oc,quarto)"),
				},
				Examples: []clihelp.Example{
					{Line: "bw init-dev", Description: "Initialize .bw.jsonc in current directory"},
					{Line: "bw init-dev -n", Description: "Dry run: preview generated config"},
					{Line: "bw init-dev --preset python", Description: "Initialize with Python/UV settings"},
					{Line: "bw init-dev -p oc", Description: "Initialize with OpenCode oc profile"},
				},
				Run: func(ctx *clihelp.Context) error {
					targetDir := "."
					if len(ctx.Args) > 0 {
						targetDir = ctx.Args[0]
					}
					var profs []string
					if profileFlag != "" {
						for _, item := range strings.Split(profileFlag, ",") {
							if trimmed := strings.TrimSpace(item); trimmed != "" {
								profs = append(profs, trimmed)
							}
						}
					}
					return cli.HandleInitDev(targetDir, forceFlag, dryRunFlag, noSSHFlag, opencodeFlag, presetFlag, profs)
				},
			},
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
					{Text: "Configuration is stored in two JSONC files: global (~/.config/bw/config.jsonc) and local (.bw.jsonc). The local config overrides the global for the current directory only. Without a subcommand, 'bw conf' shows usage information. With -g or -l, it shows the raw file contents."},
				},
				Subcommands: []clihelp.Command{
					{
						Name:        "info",
						Description: "Show the merged configuration plan (dry run)",
						UsageLine:   "bw conf info",
						Args:        clihelp.NoArgs,
						Run: func(ctx *clihelp.Context) error {
							return runConf(verboseFlag)
						},
					},
					{
						Name:        "where",
						Description: "Print paths to both config files",
						UsageLine:   "bw conf where",
						Args:        clihelp.NoArgs,
						Run: func(ctx *clihelp.Context) error {
							cli.HandleConfigWhere()
							return nil
						},
					},
					{
						Name:        "path",
						Description: "Manage sandbox PATH entries",
						UsageLine:   "bw conf path list|add|del [args...]",
						Subcommands: []clihelp.Command{
							{
								Name:        "list",
								Aliases:     []string{"ls"},
								Description: "List combined sandbox PATH entries from global and local configs",
								UsageLine:   "bw conf path list",
								Args:        clihelp.NoArgs,
								Run: func(ctx *clihelp.Context) error {
									cli.HandlePathList()
									return nil
								},
							},
							{
								Name:        "add",
								Description: "Add a directory to the sandbox PATH",
								UsageLine:   "bw conf path add <directory> -g | -l",
								Args:        clihelp.ExactArgs(1),
								Run: func(ctx *clihelp.Context) error {
									cli.HandlePathAdd(ctx.Args[0], globalFlag, localFlag)
									return nil
								},
							},
							{
								Name:        "del",
								Description: "Remove a directory from the sandbox PATH",
								UsageLine:   "bw conf path del <directory> -g | -l",
								Args:        clihelp.ExactArgs(1),
								Run: func(ctx *clihelp.Context) error {
									cli.HandlePathDel(ctx.Args[0], globalFlag, localFlag)
									return nil
								},
							},
						},
						Run: func(ctx *clihelp.Context) error {
							cli.PrintConfPathUsage()
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
								// Default to showing global config if no flag is provided
								cli.HandleConfigShowGlobal()
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
					// Show usage message when no subcommand is provided
					cli.PrintConfUsage()
					return nil
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
				Name:        "exec",
				Description: "Run an arbitrary command inside the sandbox and exit",
				UsageLine:   "bw exec <command> [args...]",
				Run: func(ctx *clihelp.Context) error {
					return runExec(ctx.Args, forceFlag, verboseFlag)
				},
			},
			{
				Name:        "profile",
				Aliases:     []string{"prof"},
				Description: "Manage, search, and generate tool capability profiles",
				UsageLine:   "bw profile <subcommand>",
				Subcommands: []clihelp.Command{
					{
						Name:        "list",
						Aliases:     []string{"ls"},
						Description: "List all registered sandbox profiles",
						UsageLine:   "bw profile list",
						Args:        clihelp.NoArgs,
						Run: func(ctx *clihelp.Context) error {
							return cli.HandleProfileList()
						},
					},
					{
						Name:        "show",
						Aliases:     []string{"info", "view", "cat"},
						Description: "Show details, mounts, and smoke tests for a profile",
						UsageLine:   "bw profile show <name>",
						Args:        clihelp.ExactArgs(1),
						Run: func(ctx *clihelp.Context) error {
							return cli.HandleProfileShow(ctx.Args[0])
						},
					},
					{
						Name:        "new",
						Aliases:     []string{"create", "add", "gen"},
						Description: "Synthesize a profile from Homebrew and Firejail intelligence",
						UsageLine:   "bw profile new <name> [-g | -l]",
						Args:        clihelp.ExactArgs(1),
						Run: func(ctx *clihelp.Context) error {
							return cli.HandleProfileNew(ctx.Args[0], globalFlag, localFlag)
						},
					},
					{
						Name:        "fetch",
						Aliases:     []string{"pull", "get", "install"},
						Description: "Download a profile from GitHub repository or synthesize from Homebrew",
						UsageLine:   "bw profile fetch <name> [-g | -l]",
						Args:        clihelp.ExactArgs(1),
						Run: func(ctx *clihelp.Context) error {
							return cli.HandleProfileFetch(ctx.Args[0], globalFlag, localFlag)
						},
					},
					{
						Name:        "update",
						Aliases:     []string{"sync"},
						Description: "Update all installed global profiles from the remote repository",
						UsageLine:   "bw profile update",
						Args:        clihelp.NoArgs,
						Run: func(ctx *clihelp.Context) error {
							return cli.HandleProfileUpdate()
						},
					},
					{
						Name:        "test",
						Description: "Run all verification and smoke tests for a profile inside sandbox",
						UsageLine:   "bw profile test <name>",
						Args:        clihelp.ExactArgs(1),
						Run: func(ctx *clihelp.Context) error {
							return cli.HandleProfileTest(ctx.Args[0], verboseFlag)
						},
					},
					{
						Name:        "search",
						Aliases:     []string{"find"},
						Description: "Search profiles, host executables, and Homebrew formulae",
						UsageLine:   "bw profile search <query>",
						Args:        clihelp.ExactArgs(1),
						Run: func(ctx *clihelp.Context) error {
							return cli.HandleProfileSearch(ctx.Args[0])
						},
					},
				},
			},
			{},
			{
				Name:        "test",
				Description: "Run a verification of a tool/profile inside the sandbox and exit",
				UsageLine:   "bw test <target>",
				Args:        clihelp.ExactArgs(1),
				Run: func(ctx *clihelp.Context) error {
					return cli.HandleProfileTest(ctx.Args[0], verboseFlag)
				},
			},
		},
		Run: func(ctx *clihelp.Context) error {
			return runDefault(ctx.Args, forceFlag, verboseFlag)
		},
	}

	rawArgs := os.Args[1:]
	normalizedArgs := make([]string, 0, len(rawArgs))
	for _, arg := range rawArgs {
		switch arg {
		case "help", "-help", "--h", "-?", "-H":
			normalizedArgs = append(normalizedArgs, "--help")
		default:
			normalizedArgs = append(normalizedArgs, arg)
		}
	}

	if err := app.Execute(normalizedArgs); err != nil {
		clihelp.PrintError(err)
		os.Exit(1)
	}
}
