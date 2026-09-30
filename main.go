package main

import (
	"errors"
	"os"
	"os/exec"
	"strings"

	"github.com/sarielhp/clihelp"
)

var Version = "0.3.64"

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
			clihelp.Bool(&f.noInit, "--no-init", false, "Skip auto-configuration of workspace when entering uninitialized directory"),
			clihelp.Bool(&f.noFileLimit, "--no-file-limit", false, "Disable the workspace file count safety check"),
			clihelp.Int(&f.maxFileCount, "--max-file-count <N>", 0, "Override the workspace file count limit (-1 to disable)"),
			clihelp.Bool(&f.tmux, "--tmux", false, "Force launching an internal tmux session even when running inside host tmux"),
			clihelp.Bool(&f.noTmux, "--no-tmux", false, "Launch direct interactive shell instead of internal tmux session"),
			clihelp.Bool(&f.verbose, "-v, --verbose", false, "Print verbose debug information (config paths, bwrap args, etc.)"),
		},
		Commands: []clihelp.Command{
			initCmd(f),
			statusCmd(f),
			planCmd(f),
			doctorCmd(f),
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
			stackCmd(f),
			configCmd(f, glValidator),
			docsCmd(f),
		},
		Run: func(ctx *clihelp.Context) error {
			return runDefault(ctx.Args, f.force, f.verbose, policyFlags(f), f.noInit, f.tmux, f.noTmux)
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

	normalized = hoistSubcommand(normalized)
	normalized = normalizeConfigSet(normalized)
	return normalizeCommandPassThrough(normalized)
}

func hoistSubcommand(args []string) []string {
	if len(args) == 0 || !strings.HasPrefix(args[0], "-") || args[0] == "--" {
		return args
	}

	for i := 0; i < len(args); i++ {
		token := args[i]
		if token == "--" {
			return args
		}
		if token == "--max-file-count" {
			i++
			continue
		}
		if strings.HasPrefix(token, "-") {
			continue
		}
		if !isKnownSubcommand(token) {
			return args
		}

		cmdStart := i
		cmdEnd := i + 1
		if cmdEnd < len(args) && isKnownSubSubcommand(token, args[cmdEnd]) {
			cmdEnd++
		}

		precedingFlags := args[:cmdStart]
		commandTokens := args[cmdStart:cmdEnd]
		rest := args[cmdEnd:]

		result := make([]string, 0, len(args))
		result = append(result, commandTokens...)
		result = append(result, precedingFlags...)
		result = append(result, rest...)
		return result
	}
	return args
}

func isKnownSubcommand(token string) bool {
	switch token {
	case "init", "initialize", "setup", "init-dev",
		"status", "info", "current",
		"plan", "dry-run",
		"doctor", "check",
		"add",
		"rm", "remove",
		"mount", "bind", "cbind",
		"bin",
		"copy", "cp", "ccopy",
		"path",
		"git-workflow", "gw", "worktree",
		"run", "exec",
		"test",
		"learn", "record", "trace",
		"profile", "prof", "profiles",
		"stack", "stacks",
		"config", "conf", "cfg",
		"docs", "doc", "faq":
		return true
	}
	return false
}

func isKnownSubSubcommand(cmd, sub string) bool {
	switch cmd {
	case "config", "conf", "cfg":
		switch sub {
		case "show", "cat", "view", "set", "get", "unset", "edit", "where", "paths", "reset", "init", "push", "scp", "sync", "trust":
			return true
		}
	case "profile", "prof":
		switch sub {
		case "list", "ls", "search", "find", "show", "info", "view", "cat", "generate", "fetch", "update", "test", "add", "del", "save", "suggest", "review":
			return true
		}
	case "stack", "stacks":
		switch sub {
		case "list", "ls", "show", "info", "view", "save", "update", "upgrade", "pull", "suggest":
			return true
		}
	case "mount", "cbind", "bind":
		switch sub {
		case "add", "del", "list", "ro", "rw", "ls", "delete", "rm", "remove":
			return true
		}
	case "bin", "path", "copy":
		switch sub {
		case "add", "del", "list", "ls", "delete", "rm", "remove":
			return true
		}
	case "git-workflow", "gw", "worktree":
		switch sub {
		case "run", "list", "ls", "prune", "clean", "rm":
			return true
		}
	}
	return false
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
