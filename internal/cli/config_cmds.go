package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"bws/internal/config"
	"bws/internal/sandbox"
)

func HandleConfigWhere() {
	cwd, _ := os.Getwd()
	fmt.Printf("Global:          %s\n", config.GlobalPath())
	fmt.Printf("Global Skeleton: %s\n", sandbox.GlobalSkeletonDir())
	fmt.Printf("Local:           %s\n", config.FindLocalPath(cwd))
	fmt.Printf("Local Skeleton:  %s\n", sandbox.LocalSkeletonDir(cwd))
}

func HandleConfigPath() {
	HandleConfigWhere()
}

func HandleConfigShow(global bool) {
	path := configFilePath(global)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			label := "Global"
			if !global {
				label = "Local"
			}
			fmt.Printf("%s config not found at %s.\n", label, path)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Error reading config: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(string(data))
}

func HandleConfigInit(global, local bool) {
	ValidateGL(global, local)
	path := configFilePath(global)
	label := "global"
	if local {
		label = "local"
	}

	if _, err := os.Stat(path); err == nil {
		backup := path + ".bak"
		if err := os.Rename(path, backup); err != nil {
			fmt.Fprintf(os.Stderr, "Error backing up config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Backed up old %s config to %s\n", label, backup)
	}

	if global {
		config.CreateDefault(path)
		examplePath := filepath.Join(filepath.Dir(path), "example-config.jsonc")
		config.CreateExampleConfig(examplePath)
		themePath := filepath.Join(config.ConfigDir(), "theme.omp.json")
		config.CreateDefaultTheme(themePath)
		sandbox.EnsureGlobalSkeleton()
	} else {
		os.MkdirAll(filepath.Dir(path), 0755)
		os.WriteFile(path, []byte(config.ExampleConfigContent), 0644)
	}

	fmt.Printf("Created %s config: %s\n", label, path)
}

func HandleConfigEdit(global, local bool) {
	ValidateGL(global, local)
	path := configFilePath(global)
	label := "global"
	if local {
		label = "local"
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "%s config not found at %s. Run 'bw conf init' first.\n", label, path)
		os.Exit(1)
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}

	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error opening editor: %v\n", err)
		os.Exit(1)
	}
}

func HandleConfigShowGlobal() {
	HandleConfigShow(true)
}

func HandleConfigShowLocal() {
	HandleConfigShow(false)
}

func HandleConfigInitGlobal() {
	HandleConfigInit(true, false)
}

func HandleConfigInitLocal() {
	HandleConfigInit(false, true)
}

func HandleConfigEditGlobal() {
	HandleConfigEdit(true, false)
}

func HandleConfigEditLocal() {
	HandleConfigEdit(false, true)
}

// PrintConfUsage prints the detailed config subcommand help text.
func PrintConfUsage() {
	fmt.Print(`Usage: bw conf [subcommand] [options]

  Manage sandbox configuration files.

  Bubblewrap (bwrap) creates a lightweight container by mounting
  directories and files from the host into an isolated filesystem.
  Configuration is stored in two JSONC files (JSON with comments):

    Global:  ~/.config/bw/config.jsonc
    Local:   .bw.jsonc  (in the current directory)

  The global config applies to all sandbox sessions. The local config
  overrides specific settings for the current directory only. When no
  flag is specified, 'bw conf' shows the merged configuration plan.

Subcommands:
  info                       Show the merged configuration plan (dry run)

  where                      Print paths to both config files

  path list|add|del          Manage sandbox PATH entries

  init -g | -l               Regenerate a config file from default settings.
                              Requires -g or -l. Existing file is backed up
                              with a .bak suffix.

  edit -g | -l               Open a config file in $EDITOR / $VISUAL / vi.
                              Requires -g or -l.

  show -g | -l               Display the raw contents of a config file.
                              Requires -g or -l.

Flags:
  -g, --global               Target the global config file
  -l, --local                Target the local config file

Examples:
  bw conf                    Show configuration command usage
  bw conf info               Show merged configuration plan
  bw conf where              Show both config file paths
  bw conf path list          List configured PATH directories
  bw conf init -g            Reset global config to defaults
  bw conf edit -l            Edit local config in your editor
  bw conf show -g            View raw global config contents
`)
}

// PrintConfPathUsage prints the detailed path subcommand help text.
func PrintConfPathUsage() {
	fmt.Print(`Usage: bw conf path list|add|del [args...]

  Manage the PATH entries in the sandbox configuration.

Subcommands:
  list                       List configured PATH entries with [g] and [l]
  add <directory> -g | -l    Add a directory to the sandbox PATH
  del <directory> -g | -l    Remove a directory from the sandbox PATH

Flags:
  -g, --global               Target the global config file
  -l, --local                Target the local config file

Examples:
  bw conf path list          List all configured PATH entries
  bw conf path add /home/user/bin -g  Add to global PATH
  bw conf path del /home/user/bin -l  Remove from local PATH
`)
}

// PrintCopyUsage prints the detailed ccopy subcommand help text.
func PrintCopyUsage() {
	fmt.Print(`Usage: bw ccopy add|list|del [args...]

  Manage the list of programs and files that are copied from the host
  into the sandbox's persistent home directory before each launch.

  Unlike bind mounts, copied files are snapshots — they are not
  updated automatically if the source changes. This is useful for
  tools and scripts that should be available in the sandbox without
  exposing the full host filesystem.

Subcommands:
  add <path> -g | -l    Add a program or file to the copy list.
                        The path must be absolute and under your home
                        directory. It is copied to the same relative
                        location inside the sandbox's home.
                        Requires -g or -l.

  list                   Show all configured copy paths from both
                         global and local configs.

  del <path> -g | -l     Remove a path from the copy list.
                         Requires -g or -l.

Flags:
  -g, --global           Target the global config file
  -l, --local            Target the local config file

Examples:
  bw ccopy add /home/user/bin/myprog -g    Add globally
  bw ccopy add /home/user/scripts/util.sh -l  Add locally
  bw ccopy list                             Show all
  bw ccopy del /home/user/bin/myprog -g    Remove globally
`)
}

// PrintBindUsage prints the detailed cbind subcommand help text.
func PrintBindUsage() {
	fmt.Print(`Usage: bw cbind add|list|del [args...]

  Manage bind mounts between the host filesystem and the sandbox.

  A bind mount makes a host directory or file accessible at a specific
  location inside the sandbox. Unlike copying, the sandbox sees the
  actual host file — changes are visible on both sides.

    Read-write bind ('rw')   — The sandbox can read and write files.
                               Any changes inside the sandbox affect
                               the host file. Use for config files,
                               project directories, and data you want
                               to edit from inside the sandbox.

    Read-only bind ('ro')    — The sandbox can read but cannot modify
                               the host file. Use for system libraries,
                               toolchains, and reference data.

  If sandbox-path is omitted, the host path is used as the sandbox
  path, so the file appears at the same location inside the sandbox.

Subcommands:
  add <host-path> [sandbox-path] -g | -l [--ro]
                        Add a bind mount. Defaults to read-write.
                        Use --ro to make it read-only.
                        If sandbox-path is omitted, it defaults to
                        the host path.
                        Requires -g or -l.

  list                   Show all bind mounts from both global and
                         local configs, grouped by read-write and
                         read-only.

  del <host-path> -g | -l
                         Remove a bind mount by its host path.
                         Searches both read-write and read-only lists.
                         Requires -g or -l.

Flags:
  -g, --global           Target the global config file
  -l, --local            Target the local config file
  --ro                   Make the bind mount read-only (cbind add only)

Examples:
  bw cbind add /home/user/projects /projects -g    RW global bind
  bw cbind add /usr/share/dict --ro -l              RO local bind
  bw cbind list                                      Show all
  bw cbind del /home/user/projects -g               Remove global
`)
}
