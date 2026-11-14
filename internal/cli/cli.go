package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"bws/internal/config"
	"bws/internal/util"
)

func configFilePath(global bool) string {
	if global {
		return config.GlobalPath()
	}
	return config.LocalPath()
}

func formatConfigDisplay(path string, global bool) string {
	if global {
		if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(path, home) {
			return "~" + strings.TrimPrefix(path, home)
		}
		return path
	}
	if cwd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(cwd, path); err == nil && !strings.HasPrefix(rel, "../../../../../") {
			return rel
		}
	}
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

// VerifyTools reports which host tools bws uses are installed and exits when a
// critical one is missing, unless skipFail is set. tmux is critical only for
// the default interactive session.
func VerifyTools(isDefaultSession, skipFail bool) {
	type tool struct {
		cmd      string
		name     string
		desc     string
		critical bool
	}
	tools := []tool{
		{"bwrap", "Bubblewrap", "Core sandbox container runtime", true},
		{"tmux", "tmux", "Default interactive session manager", isDefaultSession},
		{"emacs", "Emacs", "Default terminal text editor inside sandbox", false},
		{"gh", "GitHub CLI (gh)", "Automated Deploy Key registration", false},
		{"git", "Git", "Version control & repository detection", false},
		{"ssh-agent", "ssh-agent", "SSH authentication agent management", false},
		{"quarto", "Quarto", "Scientific and technical publishing system", false},
		{"uv", "uv", "Fast Python package and tool manager", false},
	}

	var missingCritical, missingRecommended []tool
	for _, t := range tools {
		if !util.CommandExists(t.cmd) {
			if t.critical {
				missingCritical = append(missingCritical, t)
			} else {
				missingRecommended = append(missingRecommended, t)
			}
		}
	}

	if len(missingRecommended) > 0 {
		fmt.Fprintf(os.Stderr, "[bws] Warning: Recommended tool(s) missing on host:\n")
		for _, t := range missingRecommended {
			fmt.Fprintf(os.Stderr, "  - %s (%s): %s\n", t.name, t.cmd, t.desc)
		}
	}

	if len(missingCritical) > 0 {
		fmt.Fprintf(os.Stderr, "Error: Required tool(s) missing on host:\n")
		for _, t := range missingCritical {
			fmt.Fprintf(os.Stderr, "  - %s (%s): %s\n", t.name, t.cmd, t.desc)
		}
		if skipFail {
			return
		}
		os.Exit(1)
	}
}

// VerifyBwrapUserns checks that bwrap can create user namespaces and otherwise
// exits with instructions for an AppArmor profile.
func VerifyBwrapUserns() {
	cmd := exec.Command("bwrap", "--ro-bind", "/", "/", "true")
	if err := cmd.Run(); err == nil {
		return
	}

	fmt.Fprintf(os.Stderr, "Error: bubblewrap ('bwrap') cannot set up user namespaces.\n")
	fmt.Fprintf(os.Stderr, "This is likely due to AppArmor or kernel sysctl restrictions.\n")
	fmt.Fprintf(os.Stderr, "Create an AppArmor profile for bubblewrap:\n")
	fmt.Fprintf(os.Stderr, "  sudo bash -c 'cat > /etc/apparmor.d/bwrap << \"EOF\"\n")
	fmt.Fprintf(os.Stderr, "    abi <abi/4.0>,\n")
	fmt.Fprintf(os.Stderr, "    include <tunables/global>\n\n")
	fmt.Fprintf(os.Stderr, "    profile bwrap /usr/bin/bwrap flags=(unconfined) {\n")
	fmt.Fprintf(os.Stderr, "      userns,\n")
	fmt.Fprintf(os.Stderr, "      include if exists <local/bwrap>\n")
	fmt.Fprintf(os.Stderr, "    }\n")
	fmt.Fprintf(os.Stderr, "  EOF'\n")
	fmt.Fprintf(os.Stderr, "  sudo apparmor_parser -r /etc/apparmor.d/bwrap\n")
	os.Exit(1)
}

// HandleSCP copies the bws configuration to a remote destination with scp.
func HandleSCP(args []string) {
	if len(args) == 0 || args[0] == "" {
		fmt.Fprintf(os.Stderr, "Error: Destination must be specified.\n")
		fmt.Fprintf(os.Stderr, "Usage: bws config push <user@host:>\n")
		os.Exit(1)
	}
	dest := args[0]

	if !util.CommandExists("scp") {
		fmt.Fprintf(os.Stderr, "Error: scp is not available on this system.\n")
		os.Exit(1)
	}

	globalConfig := config.GlobalPath()
	configDir := filepath.Dir(globalConfig)

	if _, err := os.Stat(globalConfig); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: Config file not found at %s.\n", globalConfig)
		os.Exit(1)
	}

	files := []string{globalConfig}
	themeFile := filepath.Join(configDir, "theme.omp.json")
	if _, err := os.Stat(themeFile); err == nil {
		files = append(files, themeFile)
	}

	fmt.Printf("Copying %s to %s\n", strings.Join(files, ", "), dest)
	scpArgs := append(slices.Clone(files), dest)
	cmd := exec.Command("scp", scpArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: scp failed.\n")
		os.Exit(1)
	}
	fmt.Println("Done.")
}

// HandleCopyAdd adds an absolute host path to the files copied into the
// sandbox home, in the global or local configuration.
func HandleCopyAdd(prog string, global, local bool) {
	printWorkspaceBanner(global)
	if !global && !local {
		local = true
	}
	path := configFilePath(global)

	ensureConfigFile(path)

	if !strings.HasPrefix(prog, "/") {
		fmt.Fprintf(os.Stderr, "Error: Program path must be absolute.\n")
		os.Exit(1)
	}
	prog = filepath.Clean(prog)
	homeDir := util.HomeDir()
	expanded := util.ExpandHome(prog)
	if !strings.HasPrefix(expanded, homeDir+"/") || strings.HasPrefix(expanded, homeDir+"/.sandbox") {
		fmt.Fprintf(os.Stderr, "Error: Program must be inside your home directory and not the sandbox backing store.\n")
		os.Exit(1)
	}

	cfg, err := config.LoadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	for _, p := range cfg.Copy {
		if p == prog {
			fmt.Printf("Program '%s' is already in the copy list.\n", prog)
			os.Exit(0)
		}
	}
	cfg.Copy = append(cfg.Copy, prog)
	if err := config.SetArrayValue(path, "copy", cfg.Copy); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	label := "global"
	if !global {
		label = "local"
	}
	fmt.Printf("Added '%s' to %s copy configuration (%s).\n", prog, label, formatConfigDisplay(path, global))
	if !global {
		PrintWorkspaceInfo(findWorkspaceForPath(path))
	}
}

// HandleCopyList prints the files copied into the sandbox home.
func HandleCopyList() {
	printed := false
	for _, src := range []struct{ label, path string }{
		{"Global", config.GlobalPath()},
		{"Local", config.LocalPath()},
	} {
		if _, err := os.Stat(src.path); err != nil {
			continue
		}
		cfg, err := config.LoadFile(src.path)
		if err != nil || len(cfg.Copy) == 0 {
			continue
		}
		fmt.Printf("%s copy list (%s):\n", src.label, src.path)
		for _, p := range cfg.Copy {
			fmt.Printf("  - %s\n", p)
		}
		printed = true
	}
	if !printed {
		fmt.Println("No programs currently configured in copy list.")
	}
}

// HandleCopyDel removes a path from the files copied into the sandbox home.
func HandleCopyDel(prog string, global, local bool) {
	printWorkspaceBanner(global)
	if !global && !local {
		local = true
	}
	path := configFilePath(global)

	ensureConfigFile(path)

	cfg, err := config.LoadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	found := false
	newCopy := make([]string, 0, len(cfg.Copy))
	for _, p := range cfg.Copy {
		if p == prog {
			found = true
		} else {
			newCopy = append(newCopy, p)
		}
	}
	if !found {
		fmt.Fprintf(os.Stderr, "Program '%s' not found in copy configuration.\n", prog)
		os.Exit(1)
	}
	if err := config.SetArrayValue(path, "copy", newCopy); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	label := "global"
	if !global {
		label = "local"
	}
	fmt.Printf("Removed '%s' from %s copy configuration (%s).\n", prog, label, formatConfigDisplay(path, global))
	if !global {
		PrintWorkspaceInfo(findWorkspaceForPath(path))
	}
}

// ensureConfigFile creates a default config at path if none exists and exits
// with an error if it cannot.
func ensureConfigFile(path string) {
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return
	}
	if err := config.CreateDefault(path); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating config %s: %v\n", path, err)
		os.Exit(1)
	}
}
