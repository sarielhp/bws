package main

import (
	"bws/internal/cli"

	"github.com/sarielhp/clihelp"
)

type appFlags struct {
	force    bool
	global   bool
	local    bool
	ro       bool
	verbose  bool
	dryRun   bool
	noSSH    bool
	noNet    bool
	proxy    bool
	noProxy  bool
	dbus     bool
	noDBus   bool
	opencode bool
	preset   string
	profiles []string
	docsDir  string
}

func initCmd(f *appFlags) clihelp.Command {
	profileOpt := clihelp.StringSlice(&f.profiles, "-p, --profile <name>", nil, "Explicitly include tool profile(s) (repeatable or comma-separated)")
	profileOpt.Complete = completeProfiles

	presetOpt := clihelp.Enum(&f.preset, "--preset <stack>", []string{"", "go", "python", "rust", "node", "latex", "agent", "all"}, "", "Explicitly select a preset stack (go, python, rust, node, latex, agent, all)")

	return clihelp.Command{
		Name:        "init",
		Aliases:     []string{"setup", "init-dev"},
		Group:       "Current environment",
		Description: "Initialize a hardened .bws/config.jsonc workspace configuration",
		UsageLine:   "bws init [options] [target-dir]",
		Args:        clihelp.RangeArgs(0, 1),
		Options: []clihelp.Option{
			clihelp.Bool(&f.dryRun, "-n, --dry-run", false, "Print generated configuration to stdout without writing to disk"),
			clihelp.Bool(&f.opencode, "--opencode", false, "Force inclusion of OpenCode configuration directories"),
			presetOpt,
			profileOpt,
		},
		Examples: []clihelp.Example{
			{Line: "bws init", Description: "Initialize .bws/config.jsonc in current directory"},
			{Line: "bws init -n", Description: "Dry run: preview generated configuration"},
			{Line: "bws init --preset python", Description: "Initialize with Python/UV settings"},
			{Line: "bws init -p node,git", Description: "Initialize with specific profiles"},
		},
		Run: func(ctx *clihelp.Context) error {
			targetDir := "."
			if len(ctx.Args) > 0 {
				targetDir = ctx.Args[0]
			}
			return cli.HandleInit(targetDir, f.force, f.dryRun, f.noSSH, f.opencode, f.preset, f.profiles)
		},
	}
}

func statusCmd(f *appFlags) clihelp.Command {
	return clihelp.Command{
		Name:        "status",
		Aliases:     []string{"info", "current"},
		Group:       "Current environment",
		Description: "Show active sandbox environment status and installed profiles",
		UsageLine:   "bws status [all]",
		Args:        clihelp.RangeArgs(0, 1),
		Examples: []clihelp.Example{
			{Line: "bws status", Description: "Show installed profiles and workspace status"},
			{Line: "bws status all", Description: "Show complete execution plan, mounts, and environment"},
		},
		Run: func(ctx *clihelp.Context) error {
			showAll := len(ctx.Args) > 0 && (ctx.Args[0] == "all" || ctx.Args[0] == "-a" || ctx.Args[0] == "--all")
			return runStatus(showAll, f.verbose)
		},
	}
}

func planCmd(f *appFlags) clihelp.Command {
	return clihelp.Command{
		Name:        "plan",
		Group:       "Current environment",
		Description: "Show complete resolved sandbox execution plan, mounts, and variables",
		UsageLine:   "bws plan",
		Args:        clihelp.NoArgs,
		Run: func(ctx *clihelp.Context) error {
			return runConf(f.verbose)
		},
	}
}

func addCmd(f *appFlags, glValidator clihelp.OptionsValidator) clihelp.Command {
	return clihelp.Command{
		Name:             "add",
		Aliases:          []string{"enable"},
		Group:            "Current environment",
		Description:      "Add one or more capability profiles to the current environment",
		UsageLine:        "bws add <name...> [-g | -l]",
		Args:             clihelp.MinimumNArgs(1),
		OptionsValidator: glValidator,
		Examples: []clihelp.Example{
			{Line: "bws add python", Description: "Enable python profile in current workspace"},
			{Line: "bws add python node rust", Description: "Enable multiple profiles at once"},
			{Line: "bws add docker -g", Description: "Enable docker profile in global config"},
		},
		Run: func(ctx *clihelp.Context) error {
			cli.HandleProfileAdd(ctx.Args, f.global, f.local)
			return nil
		},
	}
}

func rmCmd(f *appFlags, glValidator clihelp.OptionsValidator) clihelp.Command {
	return clihelp.Command{
		Name:             "rm",
		Aliases:          []string{"del", "remove", "disable"},
		Group:            "Current environment",
		Description:      "Remove one or more capability profiles from the current environment",
		UsageLine:        "bws rm <name...> [-g | -l]",
		Args:             clihelp.MinimumNArgs(1),
		OptionsValidator: glValidator,
		Examples: []clihelp.Example{
			{Line: "bws rm python", Description: "Disable python profile in current workspace"},
			{Line: "bws rm node rust", Description: "Disable multiple profiles at once"},
		},
		Run: func(ctx *clihelp.Context) error {
			cli.HandleProfileDel(ctx.Args, f.global, f.local)
			return nil
		},
	}
}

func mountCmd(f *appFlags, glValidator clihelp.OptionsValidator) clihelp.Command {
	return clihelp.Command{
		Name:        "mount",
		Aliases:     []string{"cbind", "bind"},
		Group:       "Current environment",
		Description: "Manage bind mounts between the host and the sandbox",
		UsageLine:   "bws mount add|rm|list [args...]",
		Subcommands: []clihelp.Command{
			{
				Name:             "add",
				Aliases:          []string{"enable"},
				Description:      "Add a bind mount (-g for global, defaults to local workspace; --ro for read-only)",
				UsageLine:        "bws mount add <host-path> [sandbox-path] [-g | -l] [--ro]",
				Args:             clihelp.RangeArgs(1, 2),
				OptionsValidator: glValidator,
				Options: []clihelp.Option{
					clihelp.Bool(&f.ro, "--ro", false, "Make the bind mount read-only"),
				},
				Notes: []clihelp.Note{
					{Heading: "Mount semantics", Text: "A bind mount makes a host directory or file accessible inside the sandbox. Read-write (default): the sandbox can modify the host file. Read-only (--ro): the sandbox can only read. If sandbox-path is omitted, the host path is used as-is inside the sandbox."},
				},
				Examples: []clihelp.Example{
					{Line: "bws mount add /home/user/projects /projects", Description: "Add read-write mount to local config"},
					{Line: "bws mount add /usr/share/dict --ro -g", Description: "Add read-only mount to global config"},
				},
				Run: func(ctx *clihelp.Context) error {
					hostPath := ctx.Args[0]
					sandboxPath := ""
					if len(ctx.Args) > 1 {
						sandboxPath = ctx.Args[1]
					}
					cli.HandleMountAdd(hostPath, sandboxPath, f.ro, f.global, f.local)
					return nil
				},
			},
			{
				Name:        "list",
				Aliases:     []string{"ls"},
				Description: "List all configured bind mounts from both configs",
				UsageLine:   "bws mount list",
				Args:        clihelp.NoArgs,
				Run: func(ctx *clihelp.Context) error {
					cli.HandleMountList()
					return nil
				},
			},
			{
				Name:             "del",
				Aliases:          []string{"delete", "rm", "remove"},
				Description:      "Remove a bind mount by host path",
				UsageLine:        "bws mount rm <host-path> [-g | -l]",
				Args:             clihelp.ExactArgs(1),
				OptionsValidator: glValidator,
				Examples: []clihelp.Example{
					{Line: "bws mount rm /home/user/projects", Description: "Remove local mount"},
				},
				Run: func(ctx *clihelp.Context) error {
					cli.HandleMountDel(ctx.Args[0], f.global, f.local)
					return nil
				},
			},
		},
	}
}

func copyCmd(f *appFlags, glValidator clihelp.OptionsValidator) clihelp.Command {
	return clihelp.Command{
		Name:        "copy",
		Aliases:     []string{"ccopy"},
		Group:       "Current environment",
		Description: "Manage files and programs copied into the sandbox home directory",
		UsageLine:   "bws copy add|rm|list [args...]",
		Notes: []clihelp.Note{
			{Heading: "Copied files vs bind mounts", Text: "Copied files are snapshots of host files placed into the sandbox home before each launch. Unlike bind mounts, they are not live — changes on the host after the copy are not reflected in the sandbox. This is useful for tools and scripts that should be available without exposing the full host filesystem."},
		},
		Subcommands: []clihelp.Command{
			{
				Name:             "add",
				Description:      "Add a program or file to the copy list",
				UsageLine:        "bws copy add <absolute-path> [-g | -l]",
				Args:             clihelp.ExactArgs(1),
				OptionsValidator: glValidator,
				Examples: []clihelp.Example{
					{Line: "bws copy add /home/user/bin/myprog", Description: "Add to local copy list"},
					{Line: "bws copy add /home/user/scripts/util.sh -g", Description: "Add to global copy list"},
				},
				Run: func(ctx *clihelp.Context) error {
					cli.HandleCopyAdd(ctx.Args[0], f.global, f.local)
					return nil
				},
			},
			{
				Name:        "list",
				Aliases:     []string{"ls"},
				Description: "List all configured copy paths from both configs",
				UsageLine:   "bws copy list",
				Args:        clihelp.NoArgs,
				Run: func(ctx *clihelp.Context) error {
					cli.HandleCopyList()
					return nil
				},
			},
			{
				Name:             "del",
				Aliases:          []string{"delete", "rm", "remove"},
				Description:      "Remove a path from the copy list",
				UsageLine:        "bws copy rm <absolute-path> [-g | -l]",
				Args:             clihelp.ExactArgs(1),
				OptionsValidator: glValidator,
				Examples: []clihelp.Example{
					{Line: "bws copy rm /home/user/bin/myprog", Description: "Remove from local copy list"},
				},
				Run: func(ctx *clihelp.Context) error {
					cli.HandleCopyDel(ctx.Args[0], f.global, f.local)
					return nil
				},
			},
		},
	}
}

func pathCmd(f *appFlags, glValidator clihelp.OptionsValidator) clihelp.Command {
	return clihelp.Command{
		Name:        "path",
		Group:       "Current environment",
		Description: "Manage extra directory entries in the sandbox PATH",
		UsageLine:   "bws path add|rm|list [args...]",
		Subcommands: []clihelp.Command{
			{
				Name:             "add",
				Description:      "Add a directory to the sandbox PATH",
				UsageLine:        "bws path add <directory> [-g | -l]",
				Args:             clihelp.ExactArgs(1),
				OptionsValidator: glValidator,
				Examples: []clihelp.Example{
					{Line: "bws path add ~/mytools/bin", Description: "Add directory to local PATH"},
					{Line: "bws path add /opt/extra/bin -g", Description: "Add directory to global PATH"},
				},
				Run: func(ctx *clihelp.Context) error {
					cli.HandlePathAdd(ctx.Args[0], f.global, f.local)
					return nil
				},
			},
			{
				Name:        "list",
				Aliases:     []string{"ls"},
				Description: "List all configured PATH entries",
				UsageLine:   "bws path list",
				Args:        clihelp.NoArgs,
				Run: func(ctx *clihelp.Context) error {
					cli.HandlePathList()
					return nil
				},
			},
			{
				Name:             "del",
				Aliases:          []string{"delete", "rm", "remove"},
				Description:      "Remove a directory from the sandbox PATH",
				UsageLine:        "bws path rm <directory> [-g | -l]",
				Args:             clihelp.ExactArgs(1),
				OptionsValidator: glValidator,
				Examples: []clihelp.Example{
					{Line: "bws path rm ~/mytools/bin", Description: "Remove directory from local PATH"},
				},
				Run: func(ctx *clihelp.Context) error {
					cli.HandlePathDel(ctx.Args[0], f.global, f.local)
					return nil
				},
			},
		},
	}
}

func runCmd(f *appFlags) clihelp.Command {
	return clihelp.Command{
		Name:        "run",
		Aliases:     []string{"exec"},
		Group:       "Execution & testing",
		Description: "Run an arbitrary command inside the sandbox and exit",
		UsageLine:   "bws run <command> [args...]",
		Args:        clihelp.MinimumNArgs(1),
		Run: func(ctx *clihelp.Context) error {
			return runExec(ctx.Args, f.force, f.verbose, f.noSSH, f.noNet, f.proxy, f.noProxy, f.dbus, f.noDBus)
		},
	}
}

func testCmd(f *appFlags) clihelp.Command {
	return clihelp.Command{
		Name:        "test",
		Group:       "Execution & testing",
		Description: "Run verification and smoke tests for a tool or profile inside sandbox",
		UsageLine:   "bws test <target>",
		Args:        clihelp.ExactArgs(1),
		Run: func(ctx *clihelp.Context) error {
			return cli.HandleProfileTest(ctx.Args[0], f.verbose)
		},
	}
}
