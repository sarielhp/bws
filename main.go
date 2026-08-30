package main

import (
	"os"

	"github.com/sarielhp/clihelp"
)

var Version = "0.3.17"

func buildApp() *clihelp.App {
	f := &appFlags{}
	glValidator := clihelp.MutuallyExclusive("global", "local")

	return &clihelp.App{
		Name:                "bws",
		Description:         "Launch a declarative, unprivileged Bubblewrap sandbox with composable profiles, SSH forwarding, X11, and shell theming.",
		Version:             Version,
		UsageLine:           "bws [options] [command | -- [args...]]",
		GlobalNote:          "Bws runs isolated unprivileged Bubblewrap sandboxes configured via JSONC and profiles catalog.",
		ConfigPath:          "~/.config/bws/config.jsonc",
		Pager:               true,
		AbbrevCommands:      true,
		InteractiveFallback: false,
		PersistentOptions: []clihelp.Option{
			clihelp.Bool(&f.force, "-f, --force", false, "Bypass the file count safety check / force overwrite"),
			clihelp.Bool(&f.global, "-g, --global", false, "Target the global config file (~/.config/bws/config.jsonc)"),
			clihelp.Bool(&f.local, "-l, --local", false, "Target the local config file (.bws/config.jsonc in current directory)"),
			clihelp.Bool(&f.noSSH, "--no-ssh", false, "Disable SSH agent forwarding and Git SSH commands"),
			clihelp.Bool(&f.noNet, "-N, --no-net, --offline", false, "Completely block network access (air-gapped network namespace)"),
			clihelp.Bool(&f.proxy, "--proxy", false, "Tunnel outbound sandbox network traffic through an in-process host proxy"),
			clihelp.Bool(&f.noProxy, "--no-proxy", false, "Disable the in-process host proxy"),
			clihelp.Bool(&f.verbose, "-v, --verbose", false, "Print verbose debug information (config paths, bwrap args, etc.)"),
		},
		Commands: []clihelp.Command{
			initCmd(f),
			statusCmd(f),
			planCmd(f),
			addCmd(f, glValidator),
			rmCmd(f, glValidator),
			mountCmd(f, glValidator),
			copyCmd(f, glValidator),
			pathCmd(f, glValidator),
			gitWorkflowCmd(f),
			runCmd(f),
			testCmd(f),
			profileCmd(f, glValidator),
			configCmd(f, glValidator),
			docsCmd(f),
		},
		Run: func(ctx *clihelp.Context) error {
			return runDefault(ctx.Args, f.force, f.verbose, f.noSSH, f.noNet, f.proxy, f.noProxy)
		},
	}
}

func main() {
	app := buildApp()

	rawArgs := os.Args[1:]
	normalizedArgs := make([]string, 0, len(rawArgs))
	if len(rawArgs) > 0 {
		switch rawArgs[0] {
		case "help", "-help", "--h", "-?", "-H":
			normalizedArgs = append(normalizedArgs, "--help")
			normalizedArgs = append(normalizedArgs, rawArgs[1:]...)
		default:
			normalizedArgs = rawArgs
		}
	}

	if err := app.Execute(normalizedArgs); err != nil {
		app.PrintError(err)
		os.Exit(1)
	}
}
