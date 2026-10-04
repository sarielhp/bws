package main

import (
	"errors"
	"os"
	"os/exec"
	"strings"

	"bws/internal/cli"

	"github.com/sarielhp/clihelp"
)

var Version = "0.3.74"

// rootPersistentOptions is the set of flags available to every command,
// organised into help-page groups.
func rootPersistentOptions(f *appFlags) []clihelp.Option {
	return []clihelp.Option{
		clihelp.Group("Configuration scope",
			clihelp.Bool(&f.global, "-g, --global", false, "Target the global config file")),
		clihelp.Group("Configuration scope",
			clihelp.Bool(&f.local, "-l, --local", false, "Target the local workspace config file")),
		clihelp.Group("Configuration scope",
			clihelp.Bool(&f.force, "-f, --force", false, "Bypass the file count safety check / force overwrite")),
		clihelp.Group("Sandbox policy",
			clihelp.Bool(&f.noSSH, "--no-ssh", false, "Disable SSH agent forwarding and Git SSH")),
		clihelp.Group("Sandbox policy",
			clihelp.Bool(&f.noNet, "-N, --no-net, --offline", false, "Block all network access")),
		clihelp.Group("Sandbox policy",
			clihelp.Bool(&f.proxy, "--proxy", false, "Tunnel sandbox traffic through a host proxy")),
		clihelp.Group("Sandbox policy",
			clihelp.Bool(&f.noProxy, "--no-proxy", false, "Disable the in-process host proxy")),
		clihelp.Group("Sandbox policy",
			clihelp.Bool(&f.dbus, "--dbus", false, "Enable filtered session D-Bus access")),
		clihelp.Group("Sandbox policy",
			clihelp.Bool(&f.noDBus, "--no-dbus", false, "Disable session D-Bus access")),
		clihelp.Group("Safety limits",
			clihelp.Bool(&f.noInit, "--no-init", false, "Skip workspace auto-configuration")),
		clihelp.Group("Safety limits",
			clihelp.Bool(&f.noFileLimit, "--no-file-limit", false, "Disable the file count safety check")),
		clihelp.Group("Safety limits",
			clihelp.Int(&f.maxFileCount, "--max-file-count <N>", 0, "Override the file count limit (-1 disables)")),
		clihelp.Group("Terminal & output",
			clihelp.Bool(&f.tmux, "--tmux", false, "Force an internal tmux session")),
		clihelp.Group("Terminal & output",
			clihelp.Bool(&f.noTmux, "--no-tmux", false, "Use a direct shell, not internal tmux")),
		clihelp.Group("Terminal & output",
			clihelp.Bool(&f.verbose, "-v, --verbose", false, "Print verbose debug information")),
		clihelp.Group("Terminal & output",
			clihelp.Bool(&f.noColor, "--no-color", false, "Disable ANSI color output")),
	}
}

func buildApp() *clihelp.App {
	f := &appFlags{}
	glValidator := clihelp.MutuallyExclusive("global", "local")

	return &clihelp.App{
		Name:           "bws",
		Description:    "Launch a declarative, unprivileged Bubblewrap sandbox with composable profiles, SSH forwarding, X11, and shell theming.",
		Version:        Version,
		UsageLine:      "bws [options] [command | -- [args...]]",
		GlobalNote:     "Bws runs isolated unprivileged Bubblewrap sandboxes configured via JSONC and profiles catalog.",
		ConfigPath:     "~/.config/bws/config.jsonc",
		AbbrevCommands: true,
		// A bare `bws [flags] [--] <command...>` runs a sandbox, so trailing
		// words are arguments for Run, not misspelled subcommands.
		Args: clihelp.MinimumNArgs(0),
		// -H is an opt-in single-letter alias for extended help; -E, --examples
		// renders the examples topic.
		ExtendedHelpFlag:   true,
		EnableExamplesFlag: true,
		// Keep an already-installed completion script and manual page current
		// across upgrades. It never creates or edits anything on its own.
		AutoRefreshIntegration: true,
		// Subcommand pages point at 'help flags' instead of re-listing every
		// global flag.
		OmitGlobalFlagsInCommands: true,
		PersistentOptions:         rootPersistentOptions(f),
		Commands: []clihelp.Command{
			initCmd(f),
			statusCmd(f),
			planCmd(f),
			doctorCmd(f),
			addCmd(f, glValidator),
			rmCmd(f, glValidator),
			undoCmd(f, glValidator),
			mountCmd(f, glValidator),
			binCmd(f, glValidator),
			copyCmd(f, glValidator),
			pathCmd(f, glValidator),
			gitWorkflowCmd(f),
			runCmd(f),
			testCmd(f),
			learnCmd(f, glValidator),
			profileCmd(f, glValidator),
			stackCmd(f),
			configCmd(f, glValidator),
			docsCmd(f),
			inventoryCommand(),
			clihelp.ManPageCommand(),
		},
		BeforeRun: func(ctx *clihelp.Context) error {
			// --no-color has to reach the renderer, not just be bound.
			ctx.App.NoColor = f.noColor
			cli.SetWorkspaceBannerColor(!f.noColor)
			return nil
		},
		Run: func(ctx *clihelp.Context) error {
			return runDefault(ctx.Args, f.force, f.verbose, policyFlags(f), f.noInit, f.tmux, f.noTmux)
		},
	}
}

// normalizeArgs applies the two argument rewrites clihelp cannot express
// declaratively: a negative integer as a positional value (config set) and the
// learn subcommand's "-- " passthrough. Help routing, leading flags and command
// abbreviations are handled natively by clihelp.
func normalizeArgs(rawArgs []string) []string {
	normalized := normalizeConfigSet(rawArgs)
	return normalizeCommandPassThrough(normalized)
}

func normalizeConfigSet(args []string) []string {
	setIdx := -1
	for i := 0; i < len(args)-1; i++ {
		cmd := args[i]
		if (cmd == "config" || cmd == "conf" || cmd == "cfg") && args[i+1] == "set" {
			setIdx = i + 1
			break
		}
	}
	if setIdx == -1 {
		return args
	}

	for _, a := range args[setIdx+1:] {
		if a == "--" {
			return args
		}
	}

	var flags []string
	var posArgs []string
	for _, a := range args[setIdx+1:] {
		if a == "-g" || a == "-l" || a == "--global" || a == "--local" {
			flags = append(flags, a)
		} else {
			posArgs = append(posArgs, a)
		}
	}

	if len(posArgs) != 2 {
		return args
	}

	key := posArgs[0]
	val := posArgs[1]
	if !strings.HasPrefix(val, "-") {
		return args
	}

	result := make([]string, 0, len(args)+1)
	result = append(result, args[:setIdx+1]...)
	result = append(result, flags...)
	result = append(result, key, "--", val)
	return result
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
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() > 0 {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
}
