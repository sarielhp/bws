package main

import (
	"bws/internal/cli"

	"github.com/sarielhp/clihelp"
)

func binCmd(f *appFlags, glValidator clihelp.OptionsValidator) clihelp.Command {
	return clihelp.Command{
		Name:        "bin",
		Group:       "Current environment",
		Description: "Manage individual host executables or scripts exposed inside the sandbox",
		UsageLine:   "bws bin add|rm|list [args...]",
		Subcommands: []clihelp.Command{
			{
				Name:             "add",
				Aliases:          []string{"enable"},
				Description:      "Expose an executable or script inside the sandbox as read-only on PATH",
				UsageLine:        "bws bin add <host-path> [-g | -l]",
				Args:             clihelp.ExactArgs(1),
				OptionsValidator: glValidator,
				Examples: []clihelp.Example{
					{Line: "bws bin add ~/bin/agy-run-wild", Description: "Expose script locally in current workspace"},
					{Line: "bws bin add /opt/tools/custom-tool -g", Description: "Expose tool globally in sandbox PATH"},
				},
				Run: func(ctx *clihelp.Context) error {
					cli.HandleBinAdd(ctx.Args[0], f.global, f.local)
					return nil
				},
			},
			{
				Name:        "list",
				Aliases:     []string{"ls"},
				Description: "List all configured executable binaries exposed in the sandbox",
				UsageLine:   "bws bin list",
				Args:        clihelp.NoArgs,
				Run: func(ctx *clihelp.Context) error {
					cli.HandleBinList()
					return nil
				},
			},
			{
				Name:             "del",
				Aliases:          []string{"delete", "rm", "remove", "disable"},
				Description:      "Remove an exposed binary or script from configuration",
				UsageLine:        "bws bin rm <name-or-path> [-g | -l]",
				Args:             clihelp.ExactArgs(1),
				OptionsValidator: glValidator,
				Examples: []clihelp.Example{
					{Line: "bws bin rm agy-run-wild", Description: "Remove local binary by name"},
					{Line: "bws bin rm ~/bin/agy-run-wild -g", Description: "Remove global binary by path"},
				},
				Run: func(ctx *clihelp.Context) error {
					cli.HandleBinDel(ctx.Args[0], f.global, f.local)
					return nil
				},
			},
		},
	}
}
