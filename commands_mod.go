package main

import (
	"bws/internal/cli"

	"github.com/sarielhp/clihelp"
)

func addCmd(f *appFlags, glValidator clihelp.OptionsValidator) clihelp.Command {
	var create bool
	return clihelp.Command{
		Name:             "add",
		Aliases:          []string{"enable"},
		Group:            "Current environment",
		Description:      "Add capability profiles to the environment",
		UsageLine:        "bws add <name...> [-g | -l] [-c | --create]",
		Args:             clihelp.MinimumNArgs(1),
		OptionsValidator: glValidator,
		Options: []clihelp.Option{
			clihelp.Bool(&create, "-c, --create", false, "Synthesize the profile if it does not exist"),
		},
		Examples: []clihelp.Example{
			{Line: "bws add python", Description: "Enable python profile in current workspace"},
			{Line: "bws add python node rust", Description: "Enable multiple profiles at once"},
			{Line: "bws add docker -g", Description: "Enable docker profile in global config"},
			{Line: "bws add -c fish", Description: "Synthesize fish profile if missing and enable it"},
		},
		Run: func(ctx *clihelp.Context) error {
			return cli.HandleProfileAdd(ctx.Args, f.global, f.local, create, f.force)
		},
	}
}

func rmCmd(f *appFlags, glValidator clihelp.OptionsValidator) clihelp.Command {
	return clihelp.Command{
		Name:             "rm",
		Aliases:          []string{"del", "remove", "disable"},
		Group:            "Current environment",
		Description:      "Remove capability profiles from the environment",
		UsageLine:        "bws rm <name...> [-g | -l]",
		Args:             clihelp.MinimumNArgs(1),
		OptionsValidator: glValidator,
		Examples: []clihelp.Example{
			{Line: "bws rm python", Description: "Disable python profile in current workspace"},
			{Line: "bws rm node rust", Description: "Disable multiple profiles at once"},
		},
		Run: func(ctx *clihelp.Context) error {
			cli.HandleProfileDel(ctx.Args, f.global, f.local)
			return nil
		},
	}
}

func mountCmd(f *appFlags, glValidator clihelp.OptionsValidator) clihelp.Command {
	return clihelp.Command{
		Name:        "mount",
		Aliases:     []string{"cbind", "bind"},
		Group:       "Current environment",
		Description: "Manage host-to-sandbox bind mounts",
		UsageLine:   "bws mount add|rm|list [args...]",
		Subcommands: []clihelp.Command{
			{
				Name:        "add",
				Aliases:     []string{"enable"},
				Description: "Add a bind mount",
				LongDescription: "Add a bind mount. Defaults to the local workspace and read-only access; " +
					"pass -g for global config, or --rw for read-write access.",
				UsageLine: "bws mount add <host-path> [sandbox-path] [-g | -l] [--rw] [--ro]",
				Args:      clihelp.RangeArgs(1, 2),
				Parameters: []clihelp.Param{
					{Name: "host-path", Description: "Host file or directory to expose", Complete: completePaths},
					{Name: "sandbox-path", Description: "Mount point inside the sandbox"},
				},
				OptionsValidator: glValidator,
				Options: []clihelp.Option{
					clihelp.Bool(&f.rw, "--rw", false, "Make the bind mount read-write"),
					clihelp.Bool(&f.ro, "--ro", false, "Make the bind mount read-only (default)"),
				},
				Notes: []clihelp.Note{
					{Heading: "Mount semantics", Text: "A bind mount makes a host directory or file accessible inside the sandbox. Read-only (default): the sandbox can only read. Read-write (--rw): the sandbox can modify the host file. If sandbox-path is omitted, the host path is used as-is inside the sandbox."},
				},
				Examples: []clihelp.Example{
					{Line: "bws mount add /usr/share/dict", Description: "Add read-only mount to local config (default)"},
					{Line: "bws mount add /home/user/projects /projects --rw", Description: "Add read-write mount to local config"},
					{Line: "bws mount add /usr/share/dict -g", Description: "Add read-only mount to global config"},
				},
				Run: func(ctx *clihelp.Context) error {
					hostPath := ctx.Args[0]
					sandboxPath := ""
					if len(ctx.Args) > 1 {
						sandboxPath = ctx.Args[1]
					}
					cli.HandleMountAdd(hostPath, sandboxPath, f.rw, f.global, f.local)
					return nil
				},
			},
			{
				Name:        "list",
				Aliases:     []string{"ls"},
				Description: "List all configured bind mounts from both configs",
				UsageLine:   "bws mount list",
				Args:        clihelp.NoArgs,
				Run: func(_ *clihelp.Context) error {
					cli.HandleMountList()
					return nil
				},
			},
			{
				Name:        "del",
				Aliases:     []string{"delete", "rm", "remove"},
				Description: "Remove a bind mount by host path",
				UsageLine:   "bws mount rm <host-path> [-g | -l]",
				Args:        clihelp.ExactArgs(1),
				Parameters: []clihelp.Param{
					{Name: "host-path", Description: "Host path of the mount to remove", Complete: completePaths},
				},
				OptionsValidator: glValidator,
				Examples: []clihelp.Example{
					{Line: "bws mount rm /home/user/projects", Description: "Remove local mount"},
				},
				Run: func(ctx *clihelp.Context) error {
					cli.HandleMountDel(ctx.Args[0], f.global, f.local)
					return nil
				},
			},
		},
	}
}

func copyCmd(f *appFlags, glValidator clihelp.OptionsValidator) clihelp.Command {
	cmd := listEditCmd(f, glValidator, listEditSpec{
		"copy", "absolute-path", "Host path to copy in", "copy list",
		[2]string{"/home/user/bin/myprog", "/home/user/scripts/util.sh"},
		"Add a program or file to the copy list", "List all configured copy paths from both configs", "Remove a path from the copy list",
		cli.HandleCopyAdd, cli.HandleCopyDel, cli.HandleCopyList,
	})
	cmd.Aliases = []string{"ccopy"}
	cmd.Description = "Manage files copied into the sandbox home"
	cmd.Notes = []clihelp.Note{
		{Heading: "Copied files vs bind mounts", Text: "Copied files are snapshots of host files placed into the sandbox home before each launch. Unlike bind mounts, they are not live — changes on the host after the copy are not reflected in the sandbox. This is useful for tools and scripts that should be available without exposing the full host filesystem."},
	}
	return cmd
}

func pathCmd(f *appFlags, glValidator clihelp.OptionsValidator) clihelp.Command {
	cmd := listEditCmd(f, glValidator, listEditSpec{
		"path", "directory", "Host directory to add", "PATH",
		[2]string{"~/mytools/bin", "/opt/extra/bin"},
		"Add a directory to the sandbox PATH", "List all configured PATH entries", "Remove a directory from the sandbox PATH",
		cli.HandlePathAdd, cli.HandlePathDel, cli.HandlePathList,
	})
	cmd.Description = "Manage extra directory entries in the sandbox PATH"
	return cmd
}

// listEditSpec describes a command that edits one list in the global or local
// configuration through add, list and del subcommands. samples are example
// values for the local and global add examples.
type listEditSpec struct {
	name, arg, argDesc, list   string
	samples                    [2]string
	addDesc, listDesc, delDesc string
	onAdd, onDel               func(value string, global, local bool)
	onList                     func()
}

// listEditCmd builds the add/list/del command tree described by s.
func listEditCmd(f *appFlags, glValidator clihelp.OptionsValidator, s listEditSpec) clihelp.Command {
	edit := func(fn func(string, bool, bool)) func(*clihelp.Context) error {
		return func(ctx *clihelp.Context) error {
			fn(ctx.Args[0], f.global, f.local)
			return nil
		}
	}
	return clihelp.Command{
		Name:      s.name,
		Group:     "Current environment",
		UsageLine: "bws " + s.name + " add|rm|list [args...]",
		Subcommands: []clihelp.Command{
			{
				Name:             "add",
				Description:      s.addDesc,
				UsageLine:        "bws " + s.name + " add <" + s.arg + "> [-g | -l]",
				Args:             clihelp.ExactArgs(1),
				Parameters:       []clihelp.Param{{Name: s.arg, Description: s.argDesc, Complete: completePaths}},
				OptionsValidator: glValidator,
				Examples: []clihelp.Example{
					{Line: "bws " + s.name + " add " + s.samples[0], Description: "Add to local " + s.list},
					{Line: "bws " + s.name + " add " + s.samples[1] + " -g", Description: "Add to global " + s.list},
				},
				Run: edit(s.onAdd),
			},
			{
				Name:        "list",
				Aliases:     []string{"ls"},
				Description: s.listDesc,
				UsageLine:   "bws " + s.name + " list",
				Args:        clihelp.NoArgs,
				Run: func(_ *clihelp.Context) error {
					s.onList()
					return nil
				},
			},
			{
				Name:             "del",
				Aliases:          []string{"delete", "rm", "remove"},
				Description:      s.delDesc,
				UsageLine:        "bws " + s.name + " rm <" + s.arg + "> [-g | -l]",
				Args:             clihelp.ExactArgs(1),
				OptionsValidator: glValidator,
				Examples: []clihelp.Example{
					{Line: "bws " + s.name + " rm " + s.samples[0], Description: "Remove from local " + s.list},
				},
				Run: edit(s.onDel),
			},
		},
	}
}

func undoCmd(f *appFlags, glValidator clihelp.OptionsValidator) clihelp.Command {
	return clihelp.Command{
		Name:        "undo",
		Aliases:     []string{"revert"},
		Group:       "Current environment",
		Description: "Restore the target config from its single-slot backup",
		LongDescription: "Restore a configuration file to its contents immediately before the most " +
			"recent bws write. A single backup is kept per config file; deeper history is not retained. " +
			"A restored local file is left untrusted until reviewed with 'bws config trust'.",
		UsageLine:        "bws undo [-g | -l]",
		Args:             clihelp.NoArgs,
		OptionsValidator: glValidator,
		Examples: []clihelp.Example{
			{Line: "bws undo", Description: "Restore the local workspace config"},
			{Line: "bws undo -g", Description: "Restore the global config"},
		},
		Run: func(_ *clihelp.Context) error {
			cli.HandleUndo(f.global, f.local)
			return nil
		},
	}
}

func runCmd(f *appFlags) clihelp.Command {
	return clihelp.Command{
		Name:        "run",
		Aliases:     []string{"exec"},
		Group:       "Execution & testing",
		Description: "Run an arbitrary command inside the sandbox and exit",
		UsageLine:   "bws run <command> [args...]",
		Args:        clihelp.MinimumNArgs(1),
		Parameters: []clihelp.Param{
			{Name: "command", Description: "Program to run inside the sandbox"},
			{Name: "args...", Description: "Arguments passed to the program", Variadic: true},
		},
		Run: func(ctx *clihelp.Context) error {
			return runExec(ctx.Args, f.force, f.verbose, policyFlags(f), f.noInit, f.tmux, f.noTmux)
		},
	}
}

func testCmd(f *appFlags) clihelp.Command {
	return clihelp.Command{
		Name:        "test",
		Group:       "Execution & testing",
		Description: "Run verification tests for a target",
		LongDescription: "Run the verification and smoke tests for a tool or profile inside the sandbox " +
			"and report each result.",
		UsageLine: "bws test <target>",
		Args:      clihelp.ExactArgs(1),
		Parameters: []clihelp.Param{
			{Name: "target", Description: "Tool or profile to verify"},
		},
		Run: func(ctx *clihelp.Context) error {
			return cli.HandleProfileTest(ctx.Args[0], f.verbose)
		},
	}
}
