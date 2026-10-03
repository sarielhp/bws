package main

import (
	"encoding/json"
	"strings"

	"github.com/sarielhp/clihelp"
)

// inventoryCommand is a hidden developer command that prints a machine-readable
// description of the command tree. tools/check_docs_drift.rb consumes it to
// verify that docs/commands.md stays in step with the CLI. It renders nothing
// a user sees and has no side effects.
func inventoryCommand() clihelp.Command {
	return clihelp.Command{
		Name:        "inventory",
		Hidden:      true,
		Description: "Print a JSON inventory of the command tree (developer)",
		UsageLine:   "bws inventory",
		Args:        clihelp.NoArgs,
		Run: func(ctx *clihelp.Context) error {
			enc := json.NewEncoder(ctx.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(buildInventory(ctx.App))
		},
	}
}

// commandInventory is the JSON shape emitted by the inventory command.
type commandInventory struct {
	App      string         `json:"app"`
	Name     string         `json:"name"`
	Commands []commandEntry `json:"commands"`
	Globals  []string       `json:"global_flags"`
}

// commandEntry is one command, its aliases, own flags, and children.
type commandEntry struct {
	Name        string         `json:"name"`
	Aliases     []string       `json:"aliases,omitempty"`
	Description string         `json:"description"`
	Flags       []string       `json:"flags,omitempty"`
	Subcommands []commandEntry `json:"subcommands,omitempty"`
}

func buildInventory(app *clihelp.App) commandInventory {
	inv := commandInventory{App: app.Name, Name: app.Name}
	for _, opt := range app.PersistentOptions {
		if opt.Hidden {
			continue
		}
		inv.Globals = append(inv.Globals, primaryLongFlag(opt.Flags))
	}
	_ = app.Walk(func(path []string, cmd *clihelp.Command) error {
		if cmd.Hidden {
			return nil
		}
		// Walk visits every node; build the nested tree from paths.
		entry := commandEntry{
			Name:        cmd.Name,
			Aliases:     append([]string(nil), cmd.Aliases...),
			Description: cmd.Description,
			Flags:       flagNames(cmd.Options),
		}
		inv.Commands = insertEntry(inv.Commands, path, entry)
		return nil
	})
	return inv
}

// insertEntry places entry at path inside roots, creating parents as needed.
func insertEntry(roots []commandEntry, path []string, entry commandEntry) []commandEntry {
	if len(path) == 0 {
		return roots
	}
	for i := range roots {
		if roots[i].Name == path[0] {
			if len(path) == 1 {
				roots[i].Description = entry.Description
				roots[i].Aliases = entry.Aliases
				roots[i].Flags = entry.Flags
				return roots
			}
			roots[i].Subcommands = insertEntry(roots[i].Subcommands, path[1:], entry)
			return roots
		}
	}
	if len(path) == 1 {
		return append(roots, entry)
	}
	return append(roots, commandEntry{Name: path[0]})
}

// flagNames lists the primary long flag of each visible option, prefixed by --.
func flagNames(opts []clihelp.Option) []string {
	var names []string
	for _, opt := range opts {
		if opt.Hidden {
			continue
		}
		names = append(names, primaryLongFlag(opt.Flags))
	}
	return names
}

// primaryLongFlag extracts the first --long spelling from an option spec, so
// "-v, --verbose" and "--offline, -N" both yield "--verbose" / "--offline".
func primaryLongFlag(spec string) string {
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		name := part
		if idx := strings.IndexAny(name, " \t"); idx >= 0 {
			name = name[:idx]
		}
		if strings.HasPrefix(name, "--") {
			return name
		}
	}
	return ""
}
