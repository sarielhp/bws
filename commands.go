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
	desc     string
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
			return runStatus(showAll, f.verbose, f.noSSH, f.noNet, f.proxy, f.noProxy, f.dbus, f.noDBus)
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
			return runConf(f.verbose, f.noSSH, f.noNet, f.proxy, f.noProxy, f.dbus, f.noDBus)
		},
	}
}
