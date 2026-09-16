package main

import (
	"bws/internal/cli"
	"bws/internal/policy"
	"fmt"
	"github.com/sarielhp/clihelp"
)

func policyFlags(f *appFlags) policy.Flags {
	return policy.Flags{NoSSH: f.noSSH, NoNet: f.noNet, Proxy: f.proxy, NoProxy: f.noProxy, DBus: f.dbus, NoDBus: f.noDBus}
}

func profileSaveCmd(f *appFlags, validator clihelp.OptionsValidator, compose bool) clihelp.Command {
	opts := cli.ProfileSaveOptions{}
	command := clihelp.Command{Name: "save", Aliases: []string{"snap", "export"}, Description: "Save effective sandbox capabilities as a reusable compound profile",
		UsageLine: "bws profile save <name> [options]", Args: clihelp.ExactArgs(1), OptionsValidator: validator}
	command.Options = []clihelp.Option{
		clihelp.String(&opts.Description, "-d, --desc <text>", "", "Description"),
		clihelp.Bool(&opts.DryRun, "-n, --dry-run", false, "Preview JSON and limitations without writing"),
		clihelp.Bool(&opts.Yes, "-y, --yes", false, "Confirm saving after reviewing the preview"),
		clihelp.Bool(&opts.Flatten, "--flatten", false, "Materialize capabilities instead of keeping dependency references"),
		clihelp.Bool(&opts.AllowMachinePaths, "--allow-machine-paths", false, "Acknowledge machine-specific absolute paths"),
		clihelp.StringSlice(&opts.Omit, "--omit <field>", nil, "Acknowledge unsupported configuration fields (repeatable)"),
		clihelp.StringSlice(&opts.Match, "--match <file>", nil, "Match project filenames (alternatives; repeatable)"),
		clihelp.Bool(&opts.NoDetect, "--no-detect", false, "Do not derive project matching rules"),
	}
	if compose {
		command.Name, command.Aliases, command.UsageLine = "compose", nil, "bws profile compose <name> --profiles <names>"
		command.Description = "Combine explicit profiles without activating them"
		p := clihelp.StringSlice(&opts.Profiles, "-p, --profiles <names>", nil, "Dependencies (repeatable or comma-separated)")
		p.Complete = completeProfiles
		command.Options = append(command.Options, p)
	}
	command.Run = func(ctx *clihelp.Context) error {
		if compose && len(opts.Profiles) == 0 {
			return fmt.Errorf("compose requires --profiles <names>")
		}
		opts.Name, opts.Local, opts.Force = ctx.Args[0], f.local, f.force
		opts.Flags = policyFlags(f)
		return cli.HandleProfileSave(opts)
	}
	return command
}

func profileSuggestCmd() clihelp.Command {
	var jsonOutput, compound bool
	return clihelp.Command{Name: "suggest", Description: "Explain matching profiles without changing configuration",
		UsageLine: "bws profile suggest [directory]", Args: clihelp.RangeArgs(0, 1),
		Options: []clihelp.Option{
			clihelp.Bool(&jsonOutput, "--json", false, "Print machine-readable suggestions"),
			clihelp.Bool(&compound, "--compound", false, "Suggest only compound profiles"),
		},
		Run: func(ctx *clihelp.Context) error {
			dir := "."
			if len(ctx.Args) > 0 {
				dir = ctx.Args[0]
			}
			return cli.HandleProfileSuggest(dir, jsonOutput, compound)
		},
	}
}

func profileReviewCmd() clihelp.Command {
	var accept bool
	return clihelp.Command{Name: "review", Description: "Review changed dependencies before updating approval fingerprints",
		UsageLine: "bws profile review <name> [--accept]", Args: clihelp.ExactArgs(1),
		Options: []clihelp.Option{clihelp.Bool(&accept, "--accept", false, "Approve displayed dependency changes")},
		Run:     func(ctx *clihelp.Context) error { return cli.HandleProfileReview(ctx.Args[0], accept) },
	}
}
