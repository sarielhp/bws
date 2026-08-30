package main

import (
	"bws/internal/cli"

	"github.com/sarielhp/clihelp"
)

type traceFlags struct {
	dryRun      bool
	write       bool
	profileName string
}

func traceCmd(f *appFlags, glValidator clihelp.OptionsValidator) clihelp.Command {
	var tf traceFlags

	return clihelp.Command{
		Name:             "trace",
		Aliases:          []string{"record", "learn"},
		Group:            "Execution & testing",
		Description:      "Trace a command dynamically to learn required mounts and sandbox features",
		UsageLine:        "bws trace [options] [--] <command> [args...]",
		Args:             clihelp.MinimumNArgs(1),
		OptionsValidator: glValidator,
		Options: []clihelp.Option{
			clihelp.Bool(&tf.dryRun, "-n, --dry-run", false, "Preview discovered mounts and features without saving"),
			clihelp.Bool(&tf.write, "-w, --write", false, "Write discovered mounts and features to configuration file"),
			clihelp.String(&tf.profileName, "-p, --profile <name>", "", "Save discovered configuration as a reusable capability profile"),
		},
		Examples: []clihelp.Example{
			{Line: "bws trace -- python train.py", Description: "Trace Python script and preview required mounts"},
			{Line: "bws trace -w -- node index.js", Description: "Trace Node.js and write discovered mounts to .bws/config.jsonc"},
			{Line: "bws trace -p myapp -- ./bin/myapp", Description: "Trace binary and generate profiles/myapp.json"},
			{Line: "bws trace -p mytool -g -- mytool build", Description: "Trace tool and save to global profiles directory"},
		},
		Run: func(ctx *clihelp.Context) error {
			return cli.HandleTrace(ctx.Args, tf.dryRun, tf.write, tf.profileName, f.global, f.local, f.force, f.verbose)
		},
	}
}
