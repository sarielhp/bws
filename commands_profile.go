package main

import (
	"sort"
	"strings"

	"bws/internal/cli"
	"bws/internal/profile"

	"github.com/sarielhp/clihelp"
)

func completeProfiles(toComplete string) []string {
	reg, err := profile.LoadRegistry("")
	if err != nil {
		return nil
	}
	var matches []string
	for name := range reg {
		if strings.HasPrefix(name, toComplete) {
			matches = append(matches, name)
		}
	}
	sort.Strings(matches)
	return matches
}

func profileCmd(f *appFlags, glValidator clihelp.OptionsValidator) clihelp.Command {
	nameArg := clihelp.ExactArgs(1)
	namesArg := clihelp.MinimumNArgs(1)

	return clihelp.Command{
		Name:        "profile",
		Aliases:     []string{"prof"},
		Group:       "Profile catalog",
		Description: "Search, inspect, and generate tool capability profiles",
		UsageLine:   "bws profile <subcommand>",
		Subcommands: []clihelp.Command{
			{
				Name:        "list",
				Aliases:     []string{"ls"},
				Description: "List all registered sandbox capability profiles",
				UsageLine:   "bws profile list",
				Args:        clihelp.NoArgs,
				Run: func(ctx *clihelp.Context) error {
					return cli.HandleProfileList()
				},
			},
			{
				Name:        "search",
				Aliases:     []string{"find"},
				Description: "Search sandbox profiles, host executables, and Homebrew formulae",
				UsageLine:   "bws profile search <query>",
				Args:        nameArg,
				Examples: []clihelp.Example{
					{Line: "bws profile search python", Description: "Find all Python-related profiles"},
					{Line: "bws profile search secret", Description: "Find hardening profiles for secrets"},
				},
				Run: func(ctx *clihelp.Context) error {
					return cli.HandleProfileSearch(ctx.Args[0])
				},
			},
			{
				Name:        "show",
				Aliases:     []string{"info", "view", "cat"},
				Description: "Show details, mounts, environment, and smoke tests for a profile",
				UsageLine:   "bws profile show <name>",
				Args:        nameArg,
				Run: func(ctx *clihelp.Context) error {
					return cli.HandleProfileShow(ctx.Args[0])
				},
			},
			{
				Name:             "generate",
				Aliases:          []string{"create", "new", "gen", "synthesize"},
				Description:      "Synthesize a profile from Homebrew and Firejail intelligence",
				UsageLine:        "bws profile generate <name> [-g | -l]",
				Args:             nameArg,
				OptionsValidator: glValidator,
				Run: func(ctx *clihelp.Context) error {
					return cli.HandleProfileNew(ctx.Args[0], f.global, f.local)
				},
			},
			{
				Name:             "fetch",
				Aliases:          []string{"pull", "get", "install"},
				Description:      "Download a profile definition from GitHub repository",
				UsageLine:        "bws profile fetch <name> [-g | -l]",
				Args:             nameArg,
				OptionsValidator: glValidator,
				Run: func(ctx *clihelp.Context) error {
					return cli.HandleProfileFetch(ctx.Args[0], f.global, f.local)
				},
			},
			{
				Name:        "update",
				Aliases:     []string{"sync"},
				Description: "Update all installed global profiles from the remote repository",
				UsageLine:   "bws profile update",
				Args:        clihelp.NoArgs,
				Run: func(ctx *clihelp.Context) error {
					return cli.HandleProfileUpdate()
				},
			},
			{
				Name:        "test",
				Description: "Run all verification and smoke tests for a profile inside sandbox",
				UsageLine:   "bws profile test <name>",
				Args:        nameArg,
				Run: func(ctx *clihelp.Context) error {
					return cli.HandleProfileTest(ctx.Args[0], f.verbose)
				},
			},
			{
				Name:             "add",
				Aliases:          []string{"enable"},
				Description:      "Add and enable capability profile(s) in local or global config",
				UsageLine:        "bws profile add <name...> [-g | -l]",
				Args:             namesArg,
				OptionsValidator: glValidator,
				Examples: []clihelp.Example{
					{Line: "bws profile add python", Description: "Enable python profile in local workspace"},
					{Line: "bws profile add python node -g", Description: "Enable multiple profiles in global config"},
				},
				Run: func(ctx *clihelp.Context) error {
					cli.HandleProfileAdd(ctx.Args, f.global, f.local)
					return nil
				},
			},
			{
				Name:             "del",
				Aliases:          []string{"rm", "remove", "disable"},
				Description:      "Remove and disable capability profile(s) from local or global config",
				UsageLine:        "bws profile rm <name...> [-g | -l]",
				Args:             namesArg,
				OptionsValidator: glValidator,
				Examples: []clihelp.Example{
					{Line: "bws profile rm python", Description: "Disable python profile in local workspace"},
				},
				Run: func(ctx *clihelp.Context) error {
					cli.HandleProfileDel(ctx.Args, f.global, f.local)
					return nil
				},
			},
			{
				Name:             "save",
				Aliases:          []string{"snap", "export"},
				Description:      "Snapshot current workspace configuration as a reusable profile",
				UsageLine:        "bws profile save <name> [-g | -l] [-f] [-d <description>]",
				Args:             nameArg,
				OptionsValidator: glValidator,
				Options: []clihelp.Option{
					clihelp.String(&f.desc, "-d, --desc <text>", "", "Custom description for the saved profile"),
				},
				Examples: []clihelp.Example{
					{Line: "bws profile save my-env", Description: "Snapshot current workspace as a global profile"},
					{Line: "bws profile save my-env -f", Description: "Force overwrite existing profile"},
					{Line: "bws profile save project-env -l", Description: "Save as a local workspace profile"},
				},
				Run: func(ctx *clihelp.Context) error {
					cli.HandleProfileSave(ctx.Args[0], f.desc, f.global, f.local, f.force)
					return nil
				},
			},
		},
	}
}
