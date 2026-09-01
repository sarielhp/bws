package cli

import (
	"fmt"
	"os"
	"strings"

	"bws/internal/config"
	"bws/internal/sandbox"
)

func PrintInfo(bwrapArgs []string, cfg *config.Config, globalPath, localPath, currentDir string) {
	fmt.Println("Bubblewrap sandbox configuration information")
	fmt.Println("===========================================")
	fmt.Println()

	if len(cfg.Profiles) > 0 {
		var colored []string
		for _, p := range cfg.Profiles {
			colored = append(colored, ColorProfile(p))
		}
		fmt.Printf("Active profiles: %s\n\n", strings.Join(colored, ", "))
	}

	rwBinds := [][2]string{}
	roBinds := [][2]string{}
	tmpfsBinds := []string{}
	specialMounts := [][2]string{}
	envVars := map[string]string{}
	flags := []string{}
	hostname := ""
	chdir := ""

	i := 0
	for i < len(bwrapArgs) {
		arg := bwrapArgs[i]
		switch arg {
		case "--bind", "--bind-try":
			if i+2 < len(bwrapArgs) {
				rwBinds = append(rwBinds, [2]string{bwrapArgs[i+1], bwrapArgs[i+2]})
				i += 3
				continue
			}
		case "--ro-bind", "--ro-bind-try":
			if i+2 < len(bwrapArgs) {
				roBinds = append(roBinds, [2]string{bwrapArgs[i+1], bwrapArgs[i+2]})
				i += 3
				continue
			}
		case "--tmpfs":
			if i+1 < len(bwrapArgs) {
				tmpfsBinds = append(tmpfsBinds, bwrapArgs[i+1])
				i += 2
				continue
			}
		case "--proc":
			if i+1 < len(bwrapArgs) {
				specialMounts = append(specialMounts, [2]string{"proc", bwrapArgs[i+1]})
				i += 2
				continue
			}
		case "--dev":
			if i+1 < len(bwrapArgs) {
				specialMounts = append(specialMounts, [2]string{"dev", bwrapArgs[i+1]})
				i += 2
				continue
			}
		case "--symlink":
			if i+2 < len(bwrapArgs) {
				specialMounts = append(specialMounts, [2]string{"symlink: " + bwrapArgs[i+1], bwrapArgs[i+2]})
				i += 3
				continue
			}
		case "--setenv":
			if i+2 < len(bwrapArgs) {
				envVars[bwrapArgs[i+1]] = bwrapArgs[i+2]
				i += 3
				continue
			}
		case "--unsetenv":
			if i+1 < len(bwrapArgs) {
				envVars[bwrapArgs[i+1]] = "<unset>"
				i += 2
				continue
			}
		case "--hostname":
			if i+1 < len(bwrapArgs) {
				hostname = bwrapArgs[i+1]
				i += 2
				continue
			}
		case "--chdir":
			if i+1 < len(bwrapArgs) {
				chdir = bwrapArgs[i+1]
				i += 2
				continue
			}
		case "--share-net", "--clearenv", "--unshare-uts", "--die-with-parent":
			flags = append(flags, arg)
		}
		i++
	}

	fmt.Println("Environment variables:")
	for _, k := range sortedKeys(envVars) {
		fmt.Printf("  %s=%s\n", k, envVars[k])
	}
	fmt.Println()

	fmt.Println("System / sandbox flags:")
	if cfg.System != nil {
		if config.GetBool(cfg, func(c *config.Config) *bool { return c.System.ShareNet }, false) && !contains(flags, "--share-net") {
			flags = append(flags, "--share-net")
		}
		if config.GetBool(cfg, func(c *config.Config) *bool { return c.System.Clearenv }, false) && !contains(flags, "--clearenv") {
			flags = append(flags, "--clearenv")
		}
		if config.GetBool(cfg, func(c *config.Config) *bool { return c.System.UnshareUTS }, false) && !contains(flags, "--unshare-uts") {
			flags = append(flags, "--unshare-uts")
		}
		if hostname == "" && cfg.System.Hostname != nil {
			hostname = *cfg.System.Hostname
		}
	}
	if !contains(flags, "--die-with-parent") {
		flags = append(flags, "--die-with-parent")
	}
	sortStrings(flags)
	fmt.Printf("  %s\n", strings.Join(flags, ", "))
	if hostname != "" {
		fmt.Printf("  hostname: %s\n", hostname)
	}
	if chdir == "" {
		chdir = currentDir
	}
	fmt.Printf("  chdir (working directory): %s\n", chdir)
	fmt.Println()

	fmt.Println("Special mounts & tmpfs:")
	for _, m := range specialMounts {
		fmt.Printf("  %-15s on %s\n", m[0], m[1])
	}
	for _, t := range tmpfsBinds {
		fmt.Printf("  %-15s on %s\n", "tmpfs", t)
	}
	fmt.Println()

	fmt.Println("Read-write bindings (host -> sandbox):")
	for _, b := range rwBinds {
		line := b[0] + " -> " + b[1]
		if len(line) > 80 {
			fmt.Printf("  %s\n    -> %s\n", b[0], b[1])
		} else {
			fmt.Printf("  %s\n", line)
		}
	}
	fmt.Println()

	fmt.Println("Read-only bindings (host -> sandbox):")
	var etcBinds, regularRO [][2]string
	for _, b := range roBinds {
		if strings.HasPrefix(b[1], "/etc/") {
			etcBinds = append(etcBinds, b)
		} else {
			regularRO = append(regularRO, b)
		}
	}
	for _, b := range regularRO {
		line := b[0] + " -> " + b[1]
		if len(line) > 80 {
			fmt.Printf("  %s\n    -> %s\n", b[0], b[1])
		} else {
			fmt.Printf("  %s\n", line)
		}
	}
	if len(etcBinds) > 0 {
		fmt.Printf("  (plus %d files from /etc/ mapped read-only individually)\n", len(etcBinds))
	}
	fmt.Println()

	fmt.Println("Files/directories copied or synced (host -> sandbox):")
	if len(sandbox.CopiedFiles) == 0 {
		if len(cfg.Copy) > 0 {
			fmt.Printf("  (dry run -- copying skipped; %d path(s) configured:)\n", len(cfg.Copy))
			for _, src := range cfg.Copy {
				fmt.Printf("  %s\n", src)
			}
		} else {
			fmt.Println("  (None)")
		}
	} else {
		for _, c := range sandbox.CopiedFiles {
			status := "[Up-to-date]"
			if c.Copied {
				status = "[Copied/Updated]"
			}
			line := c.Src + " -> " + c.Dest + " " + status
			if len(line) > 80 {
				fmt.Printf("  %s\n    -> %s %s\n", c.Src, c.Dest, status)
			} else {
				fmt.Printf("  %s\n", line)
			}
		}
	}
	fmt.Println()

	fmt.Printf("Global configuration file:\n  %s\n\n", globalPath)
	fmt.Printf("Local configuration file:\n")
	if _, err := os.Stat(localPath); err == nil {
		fmt.Printf("  %s\n", localPath)
	} else {
		fmt.Println("  None (Not present or not loaded)")
	}
	fmt.Println()
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortStrings(keys)
	return keys
}

func sortStrings(s []string) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}
