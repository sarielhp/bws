package main

import (
	"sort"
	"strings"

	"bws/internal/cli"
	"bws/internal/stack"

	"github.com/sarielhp/clihelp"
)

type appFlags struct {
	force        bool
	global       bool
	local        bool
	ro           bool
	rw           bool
	verbose      bool
	dryRun       bool
	noSSH        bool
	noNet        bool
	proxy        bool
	noProxy      bool
	dbus         bool
	noDBus       bool
	noInit       bool
	opencode     bool
	stack        string
	preset       string
	profiles     []string
	docsDir      string
	desc         string
	basic        bool
	yes          bool
	tmux         bool
	noTmux       bool
	noFileLimit  bool
	maxFileCount int
	noColor      bool
}

func completeStacks(toComplete string) []string {
	stacks, err := stack.List("")
	if err != nil {
		return nil
	}
	var matches []string
	for _, s := range stacks {
		if strings.HasPrefix(s.Name, toComplete) {
			matches = append(matches, s.Name)
		}
	}
	sort.Strings(matches)
	return matches
}

func initCmd(f *appFlags) clihelp.Command {
	profileOpt := clihelp.StringSlice(&f.profiles, "-p, --profile <name>", nil, "Include tool profile(s) (repeatable)")
	profileOpt.Complete = completeProfiles

	stackOpt := clihelp.String(&f.stack, "-s, --stack <name>", "", "Select an environment stack by name")
	stackOpt.Complete = completeStacks

	presetOpt := clihelp.Enum(&f.preset, "--preset <stack>", []string{"", "go", "python", "rust", "node", "latex", "agent", "all"}, "", "Select a preset stack")

	return clihelp.Command{
		Name:        "init",
		Aliases:     []string{"setup", "init-dev"},
		Group:       "Current environment",
		Description: "Initialize a reviewed workspace configuration",
		LongDescription: "Select profiles and initialize a reviewed .bws/config.jsonc configuration. " +
			"bws inspects the workspace, proposes detected stacks and profiles, and writes a local config only after review.",
		UsageLine: "bws init [options] [target-dir]",
		Args:      clihelp.RangeArgs(0, 1),
		Options: []clihelp.Option{
			clihelp.Bool(&f.basic, "--basic", false, "Select detected embedded tool profiles"),
			clihelp.Bool(&f.yes, "-y, --yes", false, "Confirm the selected initialization plan"),
			clihelp.Bool(&f.dryRun, "-n, --dry-run", false, "Print config to stdout without writing"),
			clihelp.Bool(&f.opencode, "--opencode", false, "Force inclusion of OpenCode config dirs"),
			stackOpt,
			presetOpt,
			profileOpt,
		},
		Examples: []clihelp.Example{
			{Line: "bws init", Description: "Initialize .bws/config.jsonc in current directory"},
			{Line: "bws init --stack go-agent", Description: "Initialize with Go agent persona stack"},
			{Line: "bws init -p go-dev -n", Description: "Preview a selected profile without writing"},
			{Line: "bws init --preset python", Description: "Initialize with Python/UV settings"},
			{Line: "bws init -p node,git", Description: "Initialize with exactly these profile selections"},
		},
		Run: func(ctx *clihelp.Context) error {
			targetDir := "."
			if len(ctx.Args) > 0 {
				targetDir = ctx.Args[0]
			}
			return cli.HandleInitOptions(cli.InitOptions{TargetDir: targetDir, Stack: f.stack, Force: f.force, DryRun: f.dryRun, OpenCode: f.opencode, Preset: f.preset, Profiles: f.profiles, Basic: f.basic, Yes: f.yes, Flags: policyFlags(f)})
		},
	}
}

func statusCmd(f *appFlags) clihelp.Command {
	return clihelp.Command{
		Name:        "status",
		Aliases:     []string{"info", "current"},
		Group:       "Current environment",
		Description: "Show environment status and installed profiles",
		LongDescription: "Show active sandbox environment status and installed profiles. " +
			"With 'all', print the complete execution plan, mounts, and environment variables.",
		UsageLine: "bws status [all]",
		Args:      clihelp.RangeArgs(0, 1),
		Examples: []clihelp.Example{
			{Line: "bws status", Description: "Show installed profiles and workspace status"},
			{Line: "bws status all", Description: "Show complete execution plan, mounts, and environment"},
		},
		Run: func(ctx *clihelp.Context) error {
			showAll := len(ctx.Args) > 0 && (ctx.Args[0] == "all" || ctx.Args[0] == "-a" || ctx.Args[0] == "--all")
			return runStatus(showAll, f.verbose, policyFlags(f))
		},
	}
}

func planCmd(f *appFlags) clihelp.Command {
	return clihelp.Command{
		Name:        "plan",
		Group:       "Current environment",
		Description: "Show the resolved sandbox execution plan",
		LongDescription: "Show the complete resolved sandbox execution plan: mounts, environment variables, " +
			"and the assembled bwrap invocation, without launching the sandbox.",
		UsageLine: "bws plan",
		Args:      clihelp.NoArgs,
		Run: func(ctx *clihelp.Context) error {
			return runConf(f.verbose, policyFlags(f))
		},
	}
}

func doctorCmd(f *appFlags) clihelp.Command {
	return clihelp.Command{
		Name:        "doctor",
		Group:       "Current environment",
		Description: "Validate environment and prerequisites",
		LongDescription: "Inspect and validate the sandbox environment, configuration, mounts, and external " +
			"prerequisites such as bwrap, the SSH agent, and required host tools.",
		UsageLine: "bws doctor",
		Args:      clihelp.NoArgs,
		Run: func(ctx *clihelp.Context) error {
			return cli.HandleDoctor(f.verbose)
		},
	}
}
