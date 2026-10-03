package main

import (
	"bws/internal/cli"

	"github.com/sarielhp/clihelp"
)

type learnFlags struct {
	dryRun      bool
	profileName string
}

func learnCmd(f *appFlags, glValidator clihelp.OptionsValidator) clihelp.Command {
	var lf learnFlags

	return clihelp.Command{
		Name:        "learn",
		Group:       "Execution & testing",
		Description: "Discover requirements from a command",
		LongDescription: "Learn the required mounts, binary PATH entries, and features dynamically by " +
			"tracing a command, then merge or save what was discovered.",
		UsageLine: "bws learn [options] [--] <command> [args...]",
		Parameters: []clihelp.Param{
			{Name: "command", Description: "Program whose requirements are traced"},
			{Name: "args...", Description: "Arguments passed to the program", Variadic: true},
		},
		OptionsValidator: glValidator,
		Options: []clihelp.Option{
			clihelp.Bool(&lf.dryRun, "-n, --dry-run", false, "Preview newly discovered additions/deltas without saving"),
			clihelp.String(&lf.profileName, "-p, --profile <name>", "", "Save discovery as a capability profile"),
		},
		Examples: []clihelp.Example{
			{Line: "bws learn -p myproject bash", Description: "Trace entire interactive shell session and save as capability profile"},
			{Line: "bws learn bash", Description: "Trace interactive session and merge discovered mounts into .bws/config.jsonc"},
			{Line: "bws learn -n bash", Description: "Preview discovered requirements from an interactive session without saving"},
			{Line: "bws learn -- python -c \"import pandas\"", Description: "Learn Python dependencies and merge into .bws/config.jsonc"},
			{Line: "bws learn -n -- pytest -k test_foo", Description: "Preview discovered delta without modifying config"},
			{Line: "bws learn -g cargo build", Description: "Learn and merge additions into global configuration"},
		},
		Run: func(ctx *clihelp.Context) error {
			if len(ctx.Args) == 0 {
				ctx.App.RenderCommand(clihelp.Options{Writer: ctx.Stdout}, "learn")
				return nil
			}
			return cli.HandleLearn(ctx.Args, lf.dryRun, lf.profileName, f.global, f.force, f.verbose)
		},
	}
}
