package cli

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"bws/internal/config"
	"bws/internal/sandbox"
)

// infoPlan is the sandbox layout read back from a bwrap argument list.
type infoPlan struct {
	rwBinds, roBinds, special [][2]string
	tmpfs, flags              []string
	env                       map[string]string
	hostname, chdir           string
}

// bwrapArity is the number of values each reported bwrap option takes.
var bwrapArity = map[string]int{
	"--bind": 2, "--bind-try": 2, "--ro-bind": 2, "--ro-bind-try": 2, "--symlink": 2, "--setenv": 2,
	"--tmpfs": 1, "--proc": 1, "--dev": 1, "--unsetenv": 1, "--hostname": 1, "--chdir": 1,
}

// PrintInfo prints the resolved sandbox plan: profiles, environment, flags,
// mounts, copied files and the configuration files in use.
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

	plan := parseBwrapPlan(bwrapArgs)
	fmt.Println("Environment variables:")
	for _, k := range sortedKeys(plan.env) {
		fmt.Printf("  %s=%s\n", k, plan.env[k])
	}
	fmt.Println()

	printSystemFlags(plan, cfg, currentDir)
	printMounts(plan)
	printCopiedFiles(cfg)

	fmt.Printf("Global configuration file:\n  %s\n\n", globalPath)
	fmt.Printf("Local configuration file:\n")
	if _, err := os.Stat(localPath); err == nil {
		fmt.Printf("  %s\n", localPath)
	} else {
		fmt.Println("  None (Not present or not loaded)")
	}
	fmt.Println()
}

// parseBwrapPlan sorts the bwrap arguments into the sections PrintInfo shows.
// An option missing its values is ignored.
func parseBwrapPlan(args []string) infoPlan {
	plan := infoPlan{env: map[string]string{}}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		n, takesValues := bwrapArity[arg]
		if !takesValues {
			if slices.Contains([]string{"--share-net", "--clearenv", "--unshare-uts", "--die-with-parent"}, arg) {
				plan.flags = append(plan.flags, arg)
			}
			continue
		}
		if i+n >= len(args) {
			continue
		}
		plan.addOption(arg, args[i+1:i+1+n])
		i += n
	}
	return plan
}

func (p *infoPlan) addOption(opt string, v []string) {
	switch opt {
	case "--bind", "--bind-try":
		p.rwBinds = append(p.rwBinds, [2]string{v[0], v[1]})
	case "--ro-bind", "--ro-bind-try":
		p.roBinds = append(p.roBinds, [2]string{v[0], v[1]})
	case "--symlink":
		p.special = append(p.special, [2]string{"symlink: " + v[0], v[1]})
	case "--setenv":
		p.env[v[0]] = v[1]
	case "--tmpfs":
		p.tmpfs = append(p.tmpfs, v[0])
	case "--proc", "--dev":
		p.special = append(p.special, [2]string{strings.TrimPrefix(opt, "--"), v[0]})
	case "--unsetenv":
		p.env[v[0]] = "<unset>"
	case "--hostname":
		p.hostname = v[0]
	case "--chdir":
		p.chdir = v[0]
	}
}

// printSystemFlags prints the namespace flags, adding those the system
// config requests, plus the hostname and working directory.
func printSystemFlags(plan infoPlan, cfg *config.Config, currentDir string) {
	fmt.Println("System / sandbox flags:")
	flags, hostname := plan.flags, plan.hostname
	if cfg.System != nil {
		for _, f := range []struct {
			flag string
			get  func(c *config.Config) *bool
		}{
			{"--share-net", func(c *config.Config) *bool { return c.System.ShareNet }},
			{"--clearenv", func(c *config.Config) *bool { return c.System.Clearenv }},
			{"--unshare-uts", func(c *config.Config) *bool { return c.System.UnshareUTS }},
		} {
			if config.GetBool(cfg, f.get, false) && !contains(flags, f.flag) {
				flags = append(flags, f.flag)
			}
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
	chdir := plan.chdir
	if chdir == "" {
		chdir = currentDir
	}
	fmt.Printf("  chdir (working directory): %s\n", chdir)
	fmt.Println()
}

// printMounts prints special mounts, tmpfs mounts and the bindings; the many
// individual /etc read-only binds are summarised as a count.
func printMounts(plan infoPlan) {
	fmt.Println("Special mounts & tmpfs:")
	for _, m := range plan.special {
		fmt.Printf("  %-15s on %s\n", m[0], m[1])
	}
	for _, t := range plan.tmpfs {
		fmt.Printf("  %-15s on %s\n", "tmpfs", t)
	}
	fmt.Println()

	fmt.Println("Read-write bindings (host -> sandbox):")
	for _, b := range plan.rwBinds {
		printMapping(b[0], b[1], "")
	}
	fmt.Println()

	fmt.Println("Read-only bindings (host -> sandbox):")
	etc := 0
	for _, b := range plan.roBinds {
		if strings.HasPrefix(b[1], "/etc/") {
			etc++
			continue
		}
		printMapping(b[0], b[1], "")
	}
	if etc > 0 {
		fmt.Printf("  (plus %d files from /etc/ mapped read-only individually)\n", etc)
	}
	fmt.Println()
}

// printCopiedFiles lists files copied into the sandbox home, or the
// configured copies when this was a dry run.
func printCopiedFiles(cfg *config.Config) {
	fmt.Println("Files/directories copied or synced (host -> sandbox):")
	switch {
	case len(sandbox.CopiedFiles) > 0:
		for _, c := range sandbox.CopiedFiles {
			status := "[Up-to-date]"
			if c.Copied {
				status = "[Copied/Updated]"
			}
			printMapping(c.Src, c.Dest, " "+status)
		}
	case len(cfg.Copy) > 0:
		fmt.Printf("  (dry run -- copying skipped; %d path(s) configured:)\n", len(cfg.Copy))
		for _, src := range cfg.Copy {
			fmt.Printf("  %s\n", src)
		}
	default:
		fmt.Println("  (None)")
	}
	fmt.Println()
}

// printMapping prints "src -> dest", wrapping the destination onto its own
// line when the pair is longer than 80 columns.
func printMapping(src, dest, suffix string) {
	line := src + " -> " + dest + suffix
	if len(line) > 80 {
		fmt.Printf("  %s\n    -> %s%s\n", src, dest, suffix)
		return
	}
	fmt.Printf("  %s\n", line)
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
