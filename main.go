package main

import (
	"os"
	"strings"

	"github.com/sarielhp/clihelp"
)

var Version = "0.3.25"

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
			clihelp.Bool(&f.dbus, "--dbus", false, "Enable filtered session D-Bus access via xdg-dbus-proxy"),
			clihelp.Bool(&f.noDBus, "--no-dbus", false, "Disable session D-Bus access"),
			clihelp.Bool(&f.verbose, "-v, --verbose", false, "Print verbose debug information (config paths, bwrap args, etc.)"),
		},
		Commands: []clihelp.Command{
			initCmd(f),
			statusCmd(f),
			planCmd(f),
			addCmd(f, glValidator),
			rmCmd(f, glValidator),
			mountCmd(f, glValidator),
			binCmd(f, glValidator),
			copyCmd(f, glValidator),
			pathCmd(f, glValidator),
			gitWorkflowCmd(f),
			runCmd(f),
			testCmd(f),
			learnCmd(f, glValidator),
			profileCmd(f, glValidator),
			configCmd(f, glValidator),
			docsCmd(f),
		},
		Run: func(ctx *clihelp.Context) error {
			return runDefault(ctx.Args, f.force, f.verbose, f.noSSH, f.noNet, f.proxy, f.noProxy, f.dbus, f.noDBus)
		},
	}
}

func normalizeArgs(rawArgs []string) []string {
	if len(rawArgs) == 0 {
		return rawArgs
	}

	normalized := make([]string, 0, len(rawArgs)+1)
	for i := 0; i < len(rawArgs); i++ {
		arg := rawArgs[i]
		switch arg {
		case "help", "-help", "--h", "-?", "-H":
			if i == 0 {
				normalized = append(normalized, "--help")
				continue
			}
		}
		normalized = append(normalized, arg)
	}

	return normalizeCommandPassThrough(normalized)
}

func normalizeCommandPassThrough(args []string) []string {
	learnIdx := -1
	for i, arg := range args {
		if arg == "learn" {
			learnIdx = i
			break
		}
	}
	if learnIdx == -1 {
		return args
	}

	var result []string
	result = append(result, args[:learnIdx+1]...)

	subArgs := args[learnIdx+1:]
	alreadyHasDashDash := false
	for _, a := range subArgs {
		if a == "--" {
			alreadyHasDashDash = true
			break
		}
	}
	if alreadyHasDashDash {
		result = append(result, subArgs...)
		return result
	}

	insertedDashDash := false
	for i := 0; i < len(subArgs); i++ {
		token := subArgs[i]
		if insertedDashDash {
			result = append(result, token)
			continue
		}

		if token == "-p" || token == "--profile" {
			result = append(result, token)
			if i+1 < len(subArgs) {
				i++
				result = append(result, subArgs[i])
			}
			continue
		}

		if strings.HasPrefix(token, "-p=") || strings.HasPrefix(token, "--profile=") {
			result = append(result, token)
			continue
		}

		if strings.HasPrefix(token, "-") {
			result = append(result, token)
			continue
		}

		result = append(result, "--", token)
		insertedDashDash = true
	}

	return result
}

func main() {
	app := buildApp()
	normalizedArgs := normalizeArgs(os.Args[1:])

	if err := app.Execute(normalizedArgs); err != nil {
		app.PrintError(err)
		os.Exit(1)
	}
}
