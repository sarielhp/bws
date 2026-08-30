package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"bws/internal/config"
	"bws/internal/util"
)

func ValidateGL(global, local bool) {
	if !global && !local {
		fmt.Fprintf(os.Stderr, "Error: -g (--global) or -l (--local) must be specified for write operations.\n")
		os.Exit(1)
	}
}

func configFilePath(global bool) string {
	if global {
		return config.GlobalPath()
	}
	return config.LocalPath()
}

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
	scpArgs := append(files, dest)
	cmd := exec.Command("scp", scpArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: scp failed.\n")
		os.Exit(1)
	}
	fmt.Println("Done.")
}

func HandleCopyAdd(prog string, global, local bool) {
	if !global && !local {
		local = true
	}
	path := configFilePath(global)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		config.CreateDefault(path)
	}

	if !strings.HasPrefix(prog, "/") {
		fmt.Fprintf(os.Stderr, "Error: Program path must be absolute.\n")
		os.Exit(1)
	}
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
	fmt.Printf("Added '%s' to %s copy configuration (%s).\n", prog, label, path)
}

func HandleCopyList() {
	globalPath := config.GlobalPath()
	localPath := config.LocalPath()
	printed := false

	if _, err := os.Stat(globalPath); err == nil {
		cfg, err := config.LoadFile(globalPath)
		if err == nil && len(cfg.Copy) > 0 {
			fmt.Printf("Global copy list (%s):\n", globalPath)
			for _, p := range cfg.Copy {
				fmt.Printf("  - %s\n", p)
			}
			printed = true
		}
	}
	if _, err := os.Stat(localPath); err == nil {
		cfg, err := config.LoadFile(localPath)
		if err == nil && len(cfg.Copy) > 0 {
			fmt.Printf("Local copy list (%s):\n", localPath)
			for _, p := range cfg.Copy {
				fmt.Printf("  - %s\n", p)
			}
			printed = true
		}
	}
	if !printed {
		fmt.Println("No programs currently configured in copy list.")
	}
}

func HandleCopyDel(prog string, global, local bool) {
	if !global && !local {
		local = true
	}
	path := configFilePath(global)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		config.CreateDefault(path)
	}

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
	fmt.Printf("Removed '%s' from %s copy configuration.\n", prog, label)
}
