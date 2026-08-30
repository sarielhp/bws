package main

import (
	"bws/internal/gitworkflow"

	"github.com/sarielhp/clihelp"
)

type gitWorkflowFlags struct {
	branch       string
	stash        bool
	allowDirty   bool
	listMerged   bool
	listUnmerged bool
	pruneAll     bool
	pruneDryRun  bool
}

func gitWorkflowCmd(f *appFlags) clihelp.Command {
	var gwFlags gitWorkflowFlags

	return clihelp.Command{
		Name:        "git-workflow",
		Aliases:     []string{"gw", "worktree"},
		Group:       "Core workflow",
		Description: "Run an isolated, disposable agent session in a temporary Git clone",
		UsageLine:   "bws git-workflow [subcommand|options] [-- command [args...]]",
		Options: []clihelp.Option{
			clihelp.String(&gwFlags.branch, "-b, --branch NAME", "", "Target branch name for the agent session"),
			clihelp.Bool(&gwFlags.stash, "--stash", false, "Automatically stash uncommitted changes before starting"),
			clihelp.Bool(&gwFlags.allowDirty, "--allow-dirty", false, "Allow starting even if working tree has uncommitted changes"),
		},
		Subcommands: []clihelp.Command{
			{
				Name:        "list",
				Aliases:     []string{"ls"},
				Description: "List all bws-agent-* branches with commit info and merge status",
				UsageLine:   "bws git-workflow list [--merged | --unmerged]",
				Args:        clihelp.NoArgs,
				Options: []clihelp.Option{
					clihelp.Bool(&gwFlags.listMerged, "--merged", false, "List only merged agent branches"),
					clihelp.Bool(&gwFlags.listUnmerged, "--unmerged", false, "List only unmerged agent branches"),
				},
				Examples: []clihelp.Example{
					{Line: "bws gw list", Description: "List all bws-agent-* branches"},
					{Line: "bws gw list --merged", Description: "List only merged bws-agent-* branches"},
					{Line: "bws gw list --unmerged", Description: "List only unmerged bws-agent-* branches"},
				},
				Run: func(ctx *clihelp.Context) error {
					return gitworkflow.List(gitworkflow.ListOptions{
						MergedOnly:   gwFlags.listMerged,
						UnmergedOnly: gwFlags.listUnmerged,
					})
				},
			},
			{
				Name:        "prune",
				Aliases:     []string{"clean", "rm"},
				Description: "Remove merged/abandoned bws-agent branches and cleanup /tmp/bws/agent_* temp dirs",
				UsageLine:   "bws git-workflow prune [-a] [-n]",
				Args:        clihelp.NoArgs,
				Options: []clihelp.Option{
					clihelp.Bool(&gwFlags.pruneAll, "-a, --all", false, "Remove all bws-agent branches, including unmerged/abandoned"),
					clihelp.Bool(&gwFlags.pruneDryRun, "-n, --dry-run", false, "Preview branches and temp directories to prune without deleting"),
				},
				Examples: []clihelp.Example{
					{Line: "bws gw prune", Description: "Prune merged agent branches and cleanup temp directories"},
					{Line: "bws gw prune -a", Description: "Prune all agent branches (including unmerged) and temp dirs"},
					{Line: "bws gw prune -n", Description: "Dry run preview of what would be pruned"},
				},
				Run: func(ctx *clihelp.Context) error {
					return gitworkflow.Prune(gitworkflow.PruneOptions{
						All:     gwFlags.pruneAll || f.force,
						DryRun:  gwFlags.pruneDryRun,
						Verbose: f.verbose,
					})
				},
			},
		},
		Examples: []clihelp.Example{
			{Line: "bws gw", Description: "Start an interactive shell in a disposable agent clone"},
			{Line: "bws gw list", Description: "List all agent branches with commit info and status"},
			{Line: "bws gw prune", Description: "Prune merged agent branches and /tmp/bws/agent_* directories"},
			{Line: "bws gw prune -a", Description: "Prune all agent branches (including unmerged) and temp dirs"},
			{Line: "bws gw agy", Description: "Run Antigravity autonomously in an isolated clone"},
			{Line: `bws gw -b fix-auth -- agy "Fix the auth token refresh"`, Description: "Run agent on a specific branch"},
			{Line: "bws gw --stash", Description: "Auto-stash dirty working tree and start agent session"},
		},
		Notes: []clihelp.Note{
			{Heading: "Security & Isolation", Text: "The session runs in a dedicated ephemeral clone (/tmp/bws/agent_XXXX) with SSH credentials disabled (--no-ssh). The agent cannot modify the host working tree or push to remote repositories. Upon exit, changes are fetched back to the host and presented with an interactive Merge/Squash/Keep/Discard menu."},
		},
		Run: func(ctx *clihelp.Context) error {
			return gitworkflow.Run(gitworkflow.Options{
				Branch:     gwFlags.branch,
				AllowDirty: gwFlags.allowDirty,
				Stash:      gwFlags.stash,
				Command:    ctx.Args,
				Verbose:    f.verbose,
			})
		},
	}
}
