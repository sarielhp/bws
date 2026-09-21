package main

import (
	"os"
	"sort"
	"strings"

	"bws/internal/cli"
	"bws/internal/profile"

	"github.com/sarielhp/clihelp"
)

func completeProfiles(toComplete string) []string {
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}
	reg, err := profile.LoadRegistry(cwd)
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
	return clihelp.Command{Name: "profile", Aliases: []string{"prof"}, Group: "Profile catalog",
		Description: "Inspect, compose, save and suggest capability profiles", UsageLine: "bws profile <subcommand>",
		Subcommands: []clihelp.Command{profileListCmd(f, glValidator),
			profileSearchCmd(f, glValidator),
			profileShowCmd(f, glValidator),
			profileGenerateCmd(f, glValidator),
			profileFetchCmd(f, glValidator),
			profileUpdateCmd(f, glValidator),
			profileTestCmd(f, glValidator),
			profileAddCmd(f, glValidator),
			profileDelCmd(f, glValidator),
			profileSaveCmd(f, glValidator, false),
			profileSaveCmd(f, glValidator, true),
			profileSuggestCmd(),
			profileReviewCmd()},
	}
}

func profileListCmd(f *appFlags, glValidator clihelp.OptionsValidator) clihelp.Command {
	var compoundOnly bool
	return clihelp.Command{
		Name:        "list",
		Aliases:     []string{"ls"},
		Description: "List all registered sandbox capability profiles",
		UsageLine:   "bws profile list",
		Args:        clihelp.NoArgs,
		Options:     []clihelp.Option{clihelp.Bool(&compoundOnly, "--compound", false, "List only compound profiles")},
		Run: func(ctx *clihelp.Context) error {
			return cli.HandleProfileList(compoundOnly)
		},
	}
}

func profileSearchCmd(f *appFlags, glValidator clihelp.OptionsValidator) clihelp.Command {
	return clihelp.Command{
		Name:        "search",
		Aliases:     []string{"find"},
		Description: "Search sandbox profiles, host executables, and Homebrew formulae",
		UsageLine:   "bws profile search <query>",
		Args:        clihelp.ExactArgs(1),
		Examples: []clihelp.Example{
			{Line: "bws profile search python", Description: "Find all Python-related profiles"},
			{Line: "bws profile search secret", Description: "Find hardening profiles for secrets"},
		},
		Run: func(ctx *clihelp.Context) error {
			return cli.HandleProfileSearch(ctx.Args[0])
		},
	}
}

func profileShowCmd(f *appFlags, glValidator clihelp.OptionsValidator) clihelp.Command {
	return clihelp.Command{
		Name:        "show",
		Aliases:     []string{"info", "view", "cat"},
		Description: "Show details, mounts, environment, and smoke tests for a profile",
		UsageLine:   "bws profile show <name>",
		Args:        clihelp.ExactArgs(1),
		Run: func(ctx *clihelp.Context) error {
			return cli.HandleProfileShow(ctx.Args[0])
		},
	}
}

func profileGenerateCmd(f *appFlags, glValidator clihelp.OptionsValidator) clihelp.Command {
	return clihelp.Command{
		Name:             "generate",
		Aliases:          []string{"create", "new", "gen", "synthesize"},
		Description:      "Synthesize a profile from Homebrew and Firejail intelligence",
		UsageLine:        "bws profile generate <name> [-g | -l]",
		Args:             clihelp.ExactArgs(1),
		OptionsValidator: glValidator,
		Run: func(ctx *clihelp.Context) error {
			return cli.HandleProfileNew(ctx.Args[0], f.global, f.local)
		},
	}
}

func profileFetchCmd(f *appFlags, glValidator clihelp.OptionsValidator) clihelp.Command {
	return clihelp.Command{
		Name:             "fetch",
		Aliases:          []string{"pull", "get", "install"},
		Description:      "Download a profile definition from GitHub repository",
		UsageLine:        "bws profile fetch <name> [-g | -l]",
		Args:             clihelp.ExactArgs(1),
		OptionsValidator: glValidator,
		Run: func(ctx *clihelp.Context) error {
			return cli.HandleProfileFetch(ctx.Args[0], f.global, f.local)
		},
	}
}

func profileUpdateCmd(f *appFlags, glValidator clihelp.OptionsValidator) clihelp.Command {
	return clihelp.Command{
		Name:        "update",
		Aliases:     []string{"sync"},
		Description: "Update all installed global profiles from the remote repository",
		UsageLine:   "bws profile update",
		Args:        clihelp.NoArgs,
		Run: func(ctx *clihelp.Context) error {
			return cli.HandleProfileUpdate()
		},
	}
}

func profileTestCmd(f *appFlags, glValidator clihelp.OptionsValidator) clihelp.Command {
	return clihelp.Command{
		Name:        "test",
		Description: "Run all verification and smoke tests for a profile inside sandbox",
		UsageLine:   "bws profile test <name>",
		Args:        clihelp.ExactArgs(1),
		Run: func(ctx *clihelp.Context) error {
			return cli.HandleProfileTest(ctx.Args[0], f.verbose)
		},
	}
}

func profileAddCmd(f *appFlags, glValidator clihelp.OptionsValidator) clihelp.Command {
	var create bool
	return clihelp.Command{
		Name:             "add",
		Aliases:          []string{"enable"},
		Description:      "Add and enable capability profile(s) in local or global config",
		UsageLine:        "bws profile add <name...> [-g | -l] [-c | --create]",
		Args:             clihelp.MinimumNArgs(1),
		OptionsValidator: glValidator,
		Options: []clihelp.Option{
			clihelp.Bool(&create, "-c, --create", false, "Automatically synthesize and create the profile if it does not exist"),
		},
		Examples: []clihelp.Example{
			{Line: "bws profile add python", Description: "Enable python profile in local workspace"},
			{Line: "bws profile add python node -g", Description: "Enable multiple profiles in global config"},
			{Line: "bws profile add -c fish", Description: "Synthesize fish profile if missing and enable it"},
		},
		Run: func(ctx *clihelp.Context) error {
			return cli.HandleProfileAdd(ctx.Args, f.global, f.local, create)
		},
	}
}

func profileDelCmd(f *appFlags, glValidator clihelp.OptionsValidator) clihelp.Command {
	return clihelp.Command{
		Name:             "del",
		Aliases:          []string{"rm", "remove", "disable"},
		Description:      "Remove and disable capability profile(s) from local or global config",
		UsageLine:        "bws profile rm <name...> [-g | -l]",
		Args:             clihelp.MinimumNArgs(1),
		OptionsValidator: glValidator,
		Examples: []clihelp.Example{
			{Line: "bws profile rm python", Description: "Disable python profile in local workspace"},
		},
		Run: func(ctx *clihelp.Context) error {
			cli.HandleProfileDel(ctx.Args, f.global, f.local)
			return nil
		},
	}
}
