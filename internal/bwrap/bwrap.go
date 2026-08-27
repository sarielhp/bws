package bwrap

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"bws/internal/config"
	"bws/internal/util"
)

func BuildArgs(cfg *config.Config, sandboxDir, currentDir string, dryRun, verbose bool) []string {
	var args []string

	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] Building bwrap argument list...\n")
	}

	args = append(args, "--tmpfs", "/etc")
	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose]   --tmpfs /etc\n")
	}

	if cfg.System != nil {
		if config.GetBool(cfg, func(c *config.Config) *bool { return c.System.ShareNet }, false) {
			args = append(args, "--share-net")
			if verbose {
				fmt.Fprintf(os.Stderr, "[verbose]   --share-net\n")
			}
		}
		if config.GetBool(cfg, func(c *config.Config) *bool { return c.System.Clearenv }, false) {
			args = append(args, "--clearenv")
			if verbose {
				fmt.Fprintf(os.Stderr, "[verbose]   --clearenv\n")
			}
		}
		if config.GetBool(cfg, func(c *config.Config) *bool { return c.System.UnshareUTS }, false) {
			args = append(args, "--unshare-uts")
			if verbose {
				fmt.Fprintf(os.Stderr, "[verbose]   --unshare-uts\n")
			}
		}
		if cfg.System.Hostname != nil {
			args = append(args, "--hostname", *cfg.System.Hostname)
			if verbose {
				fmt.Fprintf(os.Stderr, "[verbose]   --hostname %s\n", *cfg.System.Hostname)
			}
		}
	}

	homeDir := util.HomeDir()
	if sandboxDir != "" {
		if dryRun && strings.HasPrefix(sandboxDir, "<") {
			args = append(args, "--bind", sandboxDir, homeDir)
		} else if _, err := os.Stat(sandboxDir); err == nil {
			args = append(args, "--bind", sandboxDir, homeDir)
			if verbose {
				fmt.Fprintf(os.Stderr, "[verbose]   --bind %s %s (sandbox home)\n", sandboxDir, homeDir)
			}
		}
	}

	type bindItem struct {
		host string
		dest string
		ro   bool
	}
	var allBinds []bindItem

	for _, b := range cfg.BindsRO {
		dest := b.Sandbox
		if dest == "" {
			dest = b.Host
		}
		dest = strings.ReplaceAll(dest, config.HomeToken, homeDir)
		dest = util.ExpandHome(dest)
		host := util.ExpandHome(b.Host)
		if _, err := os.Stat(host); err == nil {
			allBinds = append(allBinds, bindItem{host: host, dest: dest, ro: true})
		} else if verbose {
			fmt.Fprintf(os.Stderr, "[verbose]   (skipping RO bind: %s does not exist)\n", host)
		}
	}

	for _, b := range cfg.BindsRW {
		dest := b.Sandbox
		if dest == "" {
			dest = b.Host
		}
		dest = strings.ReplaceAll(dest, config.HomeToken, homeDir)
		dest = util.ExpandHome(dest)
		host := util.ExpandHome(b.Host)
		if sandboxDir != "" && host == sandboxDir && dest == homeDir {
			continue
		}
		if _, err := os.Stat(host); err == nil {
			allBinds = append(allBinds, bindItem{host: host, dest: dest, ro: false})
		} else if verbose {
			fmt.Fprintf(os.Stderr, "[verbose]   (skipping RW bind: %s does not exist)\n", host)
		}
	}

	// Sort so parent directories are bound before child subdirectories.
	// When depth is equal, RO comes before RW so child RW sub-mounts can override parent RO mounts.
	sort.SliceStable(allBinds, func(i, j int) bool {
		depthI := strings.Count(strings.TrimRight(allBinds[i].dest, "/"), "/")
		depthJ := strings.Count(strings.TrimRight(allBinds[j].dest, "/"), "/")
		if depthI != depthJ {
			return depthI < depthJ
		}
		if len(allBinds[i].dest) != len(allBinds[j].dest) {
			return len(allBinds[i].dest) < len(allBinds[j].dest)
		}
		if allBinds[i].ro != allBinds[j].ro {
			return allBinds[i].ro // ro before rw
		}
		return allBinds[i].dest < allBinds[j].dest
	})

	for _, b := range allBinds {
		flag := "--bind"
		if b.ro {
			flag = "--ro-bind"
		}
		args = append(args, flag, b.host, b.dest)
		if verbose {
			fmt.Fprintf(os.Stderr, "[verbose]   %s %s %s\n", flag, b.host, b.dest)
		}
	}

	for _, k := range cfg.PassEnv {
		if k == "PATH" {
			continue
		}
		val := os.Getenv(k)
		if k == "LC_ALL" && val == "" {
			val = os.Getenv("LANG")
		}
		if k == "LOGNAME" && val == "" {
			val = os.Getenv("USER")
		}
		if val != "" {
			args = append(args, "--setenv", k, val)
			if verbose {
				fmt.Fprintf(os.Stderr, "[verbose]   --setenv %s=%s (passed)\n", k, val)
			}
		}
	}

	if cfg.Env != nil {
		for k, v := range cfg.Env {
			if k == "PATH" {
				continue
			}
			val := strings.ReplaceAll(v, config.HomeToken, homeDir)
			val = util.ExpandHome(val)
			if v == "@@PASS@@" || v == "@@HOST@@" {
				val = os.Getenv(k)
				if k == "LC_ALL" && val == "" {
					val = os.Getenv("LANG")
				}
				if k == "LOGNAME" && val == "" {
					val = os.Getenv("USER")
				}
			} else if strings.Contains(val, "$") {
				val = os.Expand(val, func(varName string) string {
					if envVal, ok := cfg.Env[varName]; ok && envVal != "" && envVal != v {
						return strings.ReplaceAll(envVal, config.HomeToken, homeDir)
					}
					return os.Getenv(varName)
				})
			}
			if val != "" {
				args = append(args, "--setenv", k, val)
				if verbose {
					fmt.Fprintf(os.Stderr, "[verbose]   --setenv %s=%s\n", k, val)
				}
			}
		}
	}

	if len(cfg.Path) > 0 {
		resolved := make([]string, 0, len(cfg.Path))
		for _, p := range cfg.Path {
			resolvedP := strings.ReplaceAll(p, config.HomeToken, homeDir)
			resolvedP = util.ExpandHome(resolvedP)
			resolved = append(resolved, resolvedP)
		}
		pathVal := strings.Join(resolved, ":")
		args = append(args, "--setenv", "PATH", pathVal)
		if verbose {
			fmt.Fprintf(os.Stderr, "[verbose]   --setenv PATH=%s\n", pathVal)
		}
	}

	if config.FeatureEnabled(cfg, func(f *config.FeaturesConfig) *bool { return f.EnableSSH }) {
		addSSHArgs(cfg, sandboxDir, &args, dryRun)
	}

	if config.FeatureEnabled(cfg, func(f *config.FeaturesConfig) *bool { return f.EnableX11 }) {
		addX11Args(&args)
	}

	if config.GetBool(cfg, func(c *config.Config) *bool {
		if c.Features == nil {
			return nil
		}
		return c.Features.EnableWSL
	}, true) {
		addWSLArgs(&args)
	}

	if config.FeatureEnabled(cfg, func(f *config.FeaturesConfig) *bool { return f.EnableEtcAutoBind }) {
		addEtcAutoBindArgs(&args)
	}

	addOptBind(&args)
	addQuartoBind(&args)

	hostTmp := "/tmp/bws/SANDBOX_TMP"
	if !dryRun {
		os.MkdirAll("/tmp/bws", 0755)
		var err error
		hostTmp, err = os.MkdirTemp("/tmp/bws", "sandbox_")
		if err != nil {
			hostTmp = "/tmp/bws/SANDBOX_TMP"
		}
	}

	args = append(args,
		"--bind", hostTmp, "/tmp",
		"--proc", "/proc",
		"--dev", "/dev",
		"--ro-bind-try", "/sys", "/sys",
		"--die-with-parent",
		"--bind", currentDir, currentDir,
		"--chdir", currentDir,
	)

	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose]   --bind %s /tmp\n", hostTmp)
		fmt.Fprintf(os.Stderr, "[verbose]   --proc /proc\n")
		fmt.Fprintf(os.Stderr, "[verbose]   --dev /dev\n")
		fmt.Fprintf(os.Stderr, "[verbose]   --ro-bind-try /sys /sys\n")
		fmt.Fprintf(os.Stderr, "[verbose]   --die-with-parent\n")
		fmt.Fprintf(os.Stderr, "[verbose]   --bind %s %s\n", currentDir, currentDir)
		fmt.Fprintf(os.Stderr, "[verbose]   --chdir %s\n", currentDir)
	}

	if config.FeatureEnabled(cfg, func(f *config.FeaturesConfig) *bool { return f.EnableX11 }) {
		if os.Getenv("DISPLAY") != "" {
			args = append(args, "--ro-bind-try", "/tmp/.X11-unix", "/tmp/.X11-unix")
		}
	}

	resolvPath := filepath.Join(sandboxDir, "etc", "resolv.conf")
	if _, err := os.Stat(resolvPath); err == nil {
		args = append(args, "--ro-bind", resolvPath, "/etc/resolv.conf")
	} else if _, err := os.Stat("/etc/resolv.conf"); err == nil {
		args = append(args, "--ro-bind", "/etc/resolv.conf", "/etc/resolv.conf")
	}

	for _, maskPath := range cfg.Mask {
		expanded := util.ExpandHome(maskPath)
		expanded = strings.ReplaceAll(expanded, config.HomeToken, homeDir)
		if fi, err := os.Stat(expanded); err == nil {
			if fi.IsDir() {
				args = append(args, "--tmpfs", expanded)
				if verbose {
					fmt.Fprintf(os.Stderr, "[verbose]   --tmpfs %s (masked directory)\n", expanded)
				}
			} else {
				args = append(args, "--ro-bind-try", "/dev/null", expanded)
				if verbose {
					fmt.Fprintf(os.Stderr, "[verbose]   --ro-bind-try /dev/null %s (masked file)\n", expanded)
				}
			}
		}
	}

	// Mask ~/.sandbox/deploy_keys to prevent reading private keys from disk
	deployKeysDir := filepath.Join(homeDir, ".sandbox", "deploy_keys")
	if fi, err := os.Stat(deployKeysDir); err == nil && fi.IsDir() {
		args = append(args, "--tmpfs", deployKeysDir)
		if verbose {
			fmt.Fprintf(os.Stderr, "[verbose]   --tmpfs %s (masked private deploy keys dir)\n", deployKeysDir)
		}
	}

	// Auto-mask workspace .bws configuration directory and .bws.jsonc
	wsRoot, _ := config.FindWorkspaceRoot(currentDir)
	for _, dir := range []string{currentDir, wsRoot} {
		bwsDir := filepath.Join(dir, ".bws")
		if fi, err := os.Stat(bwsDir); err == nil && fi.IsDir() {
			args = append(args, "--tmpfs", bwsDir)
			if verbose {
				fmt.Fprintf(os.Stderr, "[verbose]   --tmpfs %s (masked workspace .bws config dir)\n", bwsDir)
			}
		}
		bwsFile := filepath.Join(dir, ".bws.jsonc")
		if fi, err := os.Stat(bwsFile); err == nil && !fi.IsDir() {
			args = append(args, "--ro-bind-try", "/dev/null", bwsFile)
			if verbose {
				fmt.Fprintf(os.Stderr, "[verbose]   --ro-bind-try /dev/null %s (masked workspace .bws.jsonc config file)\n", bwsFile)
			}
		}
	}

	return args
}
