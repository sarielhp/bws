package main

import (
	"bws/internal/cli"

	"github.com/sarielhp/clihelp"
)

func stackCmd(f *appFlags) clihelp.Command {
	return clihelp.Command{
		Name:        "stack",
		Group:       "Profile catalog",
		Description: "Inspect, compose, save, and update persona environment stacks",
		UsageLine:   "bws stack <subcommand>",
		Subcommands: []clihelp.Command{
			stackListCmd(),
			stackShowCmd(),
			stackSaveCmd(f),
			stackUpdateCmd(f),
		},
	}
}

func stackListCmd() clihelp.Command {
	var category string
	var jsonOutput bool
	return clihelp.Command{
		Name:        "list",
		Aliases:     []string{"ls"},
		Description: "List all registered seed and user-saved environment stacks",
		UsageLine:   "bws stack list [options]",
		Args:        clihelp.NoArgs,
		Options: []clihelp.Option{
			clihelp.String(&category, "-c, --category <cat>", "", "Filter stacks by category"),
			clihelp.Bool(&jsonOutput, "--json", false, "Print machine-readable JSON"),
		},
		Examples: []clihelp.Example{
			{Line: "bws stack list", Description: "List all registered stacks"},
			{Line: "bws stack list -c runtime", Description: "List stacks in 'runtime' category"},
			{Line: "bws stack list --json", Description: "Output stacks in JSON format"},
		},
		Run: func(ctx *clihelp.Context) error {
			return cli.HandleStackList(category, jsonOutput)
		},
	}
}

func stackShowCmd() clihelp.Command {
	return clihelp.Command{
		Name:        "show",
		Aliases:     []string{"info", "view"},
		Description: "Show details, profiles, and provenance for a stack",
		UsageLine:   "bws stack show <name>",
		Args:        clihelp.ExactArgs(1),
		Examples: []clihelp.Example{
			{Line: "bws stack show go-agent", Description: "Show details for go-agent stack"},
			{Line: "bws stack show latex-review", Description: "Show details for latex-review stack"},
		},
		Run: func(ctx *clihelp.Context) error {
			return cli.HandleStackShow(ctx.Args[0])
		},
	}
}

func stackSaveCmd(f *appFlags) clihelp.Command {
	var title string
	var desc string
	var noVerify bool
	return clihelp.Command{
		Name:        "save",
		Aliases:     []string{"snap"},
		Description: "Save the active workspace as a reusable user stack (genesis invariant)",
		UsageLine:   "bws stack save <name> [options]",
		Args:        clihelp.ExactArgs(1),
		Options: []clihelp.Option{
			clihelp.String(&title, "-t, --title <text>", "", "Human-readable title for the stack"),
			clihelp.String(&desc, "-d, --desc <text>", "", "Description of the stack persona"),
			clihelp.Bool(&noVerify, "--no-verify", false, "Bypass smoke tests for constituent profiles"),
		},
		Examples: []clihelp.Example{
			{Line: "bws stack save my-agent", Description: "Save current workspace setup as user stack"},
			{Line: "bws stack save my-agent --no-verify", Description: "Save stack bypassing automated smoke tests"},
		},
		Run: func(ctx *clihelp.Context) error {
			return cli.HandleStackSave(ctx.Args[0], title, desc, f.force, noVerify)
		},
	}
}

func stackUpdateCmd(f *appFlags) clihelp.Command {
	var dryRun bool
	var yes bool
	return clihelp.Command{
		Name:        "update",
		Aliases:     []string{"upgrade", "pull"},
		Description: "Pull upstream stack definition changes into current workspace",
		UsageLine:   "bws stack update [options]",
		Args:        clihelp.NoArgs,
		Options: []clihelp.Option{
			clihelp.Bool(&dryRun, "-n, --dry-run", false, "Preview upstream changes without applying"),
			clihelp.Bool(&yes, "-y, --yes", false, "Confirm and apply upstream changes without prompt"),
		},
		Examples: []clihelp.Example{
			{Line: "bws stack update", Description: "Pull and review upstream stack updates"},
			{Line: "bws stack update -n", Description: "Dry run preview of upstream updates"},
		},
		Run: func(ctx *clihelp.Context) error {
			return cli.HandleStackUpdate(dryRun, yes)
		},
	}
}
