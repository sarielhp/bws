package main

import (
	"fmt"

	"bws/internal/cli"

	"github.com/sarielhp/clihelp"
)

func configCmd(f *appFlags, glValidator clihelp.OptionsValidator) clihelp.Command {
	return clihelp.Command{
		Name:             "config",
		Aliases:          []string{"conf"},
		Group:            "Configuration & sync",
		Description:      "Manage global and local sandbox configuration files, sync, and completions",
		UsageLine:        "bws config [subcommand] [-g | -l]",
		OptionsValidator: glValidator,
		Notes: []clihelp.Note{
			{Heading: "Configuration architecture", Text: "Configuration is stored in two JSONC files: global (~/.config/bws/config.jsonc) and local (.bws/config.jsonc). The local config overrides the global for the current workspace only."},
		},
		Subcommands: []clihelp.Command{
			{
				Name:             "show",
				Aliases:          []string{"cat", "view"},
				Description:      "Display raw contents of global or local configuration file",
				UsageLine:        "bws config show [-g | -l]",
				Args:             clihelp.NoArgs,
				OptionsValidator: glValidator,
				Run: func(ctx *clihelp.Context) error {
					cli.HandleConfigShow(f.global, f.local)
					return nil
				},
			},
			{
				Name:             "set",
				Description:      "Set a configuration key value in local or global configuration",
				UsageLine:        "bws config set <key> <value> [-g | -l]",
				Args:             clihelp.ExactArgs(2),
				OptionsValidator: glValidator,
				Examples: []clihelp.Example{
					{Line: "bws config set enable_proxy true", Description: "Enable proxy in local workspace config"},
					{Line: "bws config set enable_ssh true -g", Description: "Enable SSH forwarding in global config"},
					{Line: "bws config set max_file_count 25000", Description: "Set file count safety limit"},
				},
				Run: func(ctx *clihelp.Context) error {
					cli.HandleConfigSet(ctx.Args[0], ctx.Args[1], f.global, f.local)
					return nil
				},
			},
			{
				Name:             "get",
				Description:      "Read a configuration key value from local or global configuration",
				UsageLine:        "bws config get <key> [-g | -l]",
				Args:             clihelp.ExactArgs(1),
				OptionsValidator: glValidator,
				Examples: []clihelp.Example{
					{Line: "bws config get enable_proxy", Description: "Get proxy setting from local config"},
					{Line: "bws config get max_file_count -g", Description: "Get file limit from global config"},
				},
				Run: func(ctx *clihelp.Context) error {
					cli.HandleConfigGet(ctx.Args[0], f.global, f.local)
					return nil
				},
			},
			{
				Name:             "unset",
				Description:      "Remove a configuration key from local or global configuration",
				UsageLine:        "bws config unset <key> [-g | -l]",
				Args:             clihelp.ExactArgs(1),
				OptionsValidator: glValidator,
				Examples: []clihelp.Example{
					{Line: "bws config unset enable_proxy", Description: "Remove proxy override from local config"},
				},
				Run: func(ctx *clihelp.Context) error {
					cli.HandleConfigUnset(ctx.Args[0], f.global, f.local)
					return nil
				},
			},
			{
				Name:             "edit",
				Description:      "Open configuration file in $EDITOR",
				UsageLine:        "bws config edit [-g | -l]",
				Args:             clihelp.NoArgs,
				OptionsValidator: glValidator,
				Run: func(ctx *clihelp.Context) error {
					cli.HandleConfigEdit(f.global, f.local)
					return nil
				},
			},
			{
				Name:        "where",
				Aliases:     []string{"paths"},
				Description: "Print filepaths of active global and local configuration files",
				UsageLine:   "bws config where",
				Args:        clihelp.NoArgs,
				Run: func(ctx *clihelp.Context) error {
					cli.HandleConfigWhere()
					return nil
				},
			},
			{
				Name:             "reset",
				Aliases:          []string{"init"},
				Description:      "Reset configuration file to clean defaults (backs up existing to .bak)",
				UsageLine:        "bws config reset [-g | -l]",
				Args:             clihelp.NoArgs,
				OptionsValidator: glValidator,
				Run: func(ctx *clihelp.Context) error {
					cli.HandleConfigReset(f.global, f.local)
					return nil
				},
			},
			{
				Name:        "push",
				Aliases:     []string{"scp", "sync"},
				Description: "Copy global configuration and themes to a remote host via SCP",
				UsageLine:   "bws config push <user@host:>",
				Args:        clihelp.ExactArgs(1),
				Examples: []clihelp.Example{
					{Line: "bws config push user@server:", Description: "Copy config and themes to remote host"},
				},
				Run: func(ctx *clihelp.Context) error {
					cli.HandleConfigPush(ctx.Args[0])
					return nil
				},
			},
			clihelp.CompletionCommand(),
		},
		Run: func(ctx *clihelp.Context) error {
			if f.global || f.local {
				cli.HandleConfigShow(f.global, f.local)
				return nil
			}
			ctx.App.Render(clihelp.Options{Writer: ctx.Stdout}, "config")
			return nil
		},
	}
}

func docsCmd(f *appFlags) clihelp.Command {
	return clihelp.Command{
		Name:        "docs",
		Hidden:      true,
		Description: "Generate Markdown documentation for all commands and options",
		UsageLine:   "bws docs [options]",
		Options: []clihelp.Option{
			clihelp.String(&f.docsDir, "-d, --dir PATH", "docs/clihelp", "Target directory for generated markdown pages"),
		},
		Run: func(ctx *clihelp.Context) error {
			changed, err := clihelp.RenderMarkdown(ctx.App, clihelp.MarkdownOptions{Dir: f.docsDir})
			if err != nil {
				return fmt.Errorf("rendering markdown docs: %w", err)
			}
			if changed {
				fmt.Printf("Generated Markdown documentation in %s\n", f.docsDir)
			} else {
				fmt.Printf("Markdown documentation in %s is up to date\n", f.docsDir)
			}
			return nil
		},
	}
}
