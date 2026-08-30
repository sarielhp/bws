package main

import (
	"bws/internal/gitworkflow"

	"github.com/sarielhp/clihelp"
)

type gitWorkflowFlags struct {
	branch     string
	stash      bool
	allowDirty bool
	verbose    bool
}

func gitWorkflowCmd(f *appFlags) clihelp.Command {
	var gwFlags gitWorkflowFlags

	return clihelp.Command{
		Name:        "git-workflow",
		Aliases:     []string{"gw", "worktree"},
		Group:       "Core workflow",
		Description: "Run an isolated, disposable agent session in a temporary Git clone",
		UsageLine:   "bws git-workflow [options] [-- command [args...]]",
		Options: []clihelp.Option{
			clihelp.String(&gwFlags.branch, "-b, --branch NAME", "", "Target branch name for the agent session"),
			clihelp.Bool(&gwFlags.stash, "--stash", false, "Automatically stash uncommitted changes before starting"),
			clihelp.Bool(&gwFlags.allowDirty, "--allow-dirty", false, "Allow starting even if working tree has uncommitted changes"),
			clihelp.Bool(&gwFlags.verbose, "-v, --verbose", false, "Enable verbose diagnostic logging"),
		},
		Examples: []clihelp.Example{
			{Line: "bws gw", Description: "Start an interactive shell in a disposable agent clone"},
			{Line: "bws gw agy", Description: "Run Antigravity autonomously in an isolated clone"},
			{Line: `bws gw -b fix-auth -- agy "Fix the auth token refresh"`, Description: "Run agent on a specific branch"},
			{Line: "bws gw --stash", Description: "Auto-stash dirty working tree and start agent session"},
		},
		Notes: []clihelp.Note{
			{Heading: "Security & Isolation", Text: "The session runs in a dedicated ephemeral clone (/tmp/bws/agent_XXXX) with SSH credentials disabled (--no-ssh). The agent cannot modify the host working tree or push to remote repositories. Upon exit, changes are fetched back to the host and presented with an interactive Merge/Squash/Keep/Discard menu."},
		},
		Run: func(ctx *clihelp.Context) error {
			verbose := gwFlags.verbose || f.verbose
			return gitworkflow.Run(gitworkflow.Options{
				Branch:     gwFlags.branch,
				AllowDirty: gwFlags.allowDirty,
				Stash:      gwFlags.stash,
				Command:    ctx.Args,
				Verbose:    verbose,
			})
		},
	}
}
