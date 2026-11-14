package main

import (
	"errors"
	"fmt"
	"os"

	"bws/internal/cli"

	"github.com/sarielhp/clihelp"
)

func setupCmd(f *appFlags) clihelp.Command {
	withOpt := clihelp.StringSlice(&f.with, "-w, --with <profiles>", nil, "Select exactly these profiles without the screen")
	withOpt.Complete = completeProfiles
	return clihelp.Command{
		Name:        "setup",
		Group:       "Start here",
		Description: "Choose languages, editors, agents and access",
		LongDescription: "Open a full-screen chooser for this workspace: languages, editors, AI agents, tools, " +
			"restrictions, and access (network, git push, GUI). The details pane shows what each choice " +
			"mounts and passes in. Enter writes .bws/config.jsonc, approves it, and starts the session. " +
			"Running it again shows the current choices; settings it does not manage are kept.",
		UsageLine: "bws setup [options]",
		Args:      clihelp.NoArgs,
		Options: []clihelp.Option{
			withOpt,
			clihelp.Bool(&f.noStart, "--no-start", false, "Save the selection without starting a session"),
		},
		Examples: []clihelp.Example{
			{Line: "bws setup", Description: "Choose interactively, then start"},
			{Line: "bws setup -w latex,emacs,claude,codex --no-start", Description: "Choose without the screen"},
		},
		Run: func(ctx *clihelp.Context) error {
			err := cli.RunSetup(cli.SetupOptions{TargetDir: ".", With: f.with})
			if errors.Is(err, cli.ErrSetupCancelled) {
				fmt.Fprintln(os.Stderr, "[bws] "+err.Error())
				return nil
			}
			if err != nil || f.noStart || !cli.IsInteractiveTTY(int(os.Stdin.Fd())) {
				return err
			}
			return runDefault(nil, f.force, f.verbose, policyFlags(f), true, f.tmux, f.noTmux)
		},
	}
}
