package bwrap

import (
	"fmt"
	"os"
	"path/filepath"
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
	args = append(args, "--unshare-ipc")
	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose]   --tmpfs /etc\n")
		fmt.Fprintf(os.Stderr, "[verbose]   --unshare-ipc\n")
	}

	if config.FeatureEnabledDefault(cfg, func(f *config.FeaturesConfig) *bool { return f.NoNet }, false) ||
		config.FeatureEnabledDefault(cfg, func(f *config.FeaturesConfig) *bool { return f.UnshareNet }, false) {
		args = append(args, "--unshare-net")
		if verbose {
			fmt.Fprintf(os.Stderr, "[verbose]   --unshare-net (air-gapped network namespace)\n")
		}
	} else {
		args = append(args, "--share-net")
		if verbose {
			fmt.Fprintf(os.Stderr, "[verbose]   --share-net\n")
		}
	}

	if cfg.System != nil {
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

	args = append(args, buildBinds(cfg, sandboxDir, homeDir, currentDir, verbose)...)

	for _, pattern := range cfg.PassEnv {
		if strings.HasSuffix(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, "*")
			for _, envItem := range os.Environ() {
				parts := strings.SplitN(envItem, "=", 2)
				if len(parts) == 2 && strings.HasPrefix(parts[0], prefix) {
					args = append(args, "--setenv", parts[0], parts[1])
					if verbose {
						fmt.Fprintf(os.Stderr, "[verbose]   --setenv %s=%s (passed wildcard)\n", parts[0], parts[1])
					}
				}
			}
		} else {
			if pattern == "PATH" {
				continue
			}
			val := os.Getenv(pattern)
			if pattern == "LC_ALL" && val == "" {
				val = os.Getenv("LANG")
			}
			if pattern == "LOGNAME" && val == "" {
				val = os.Getenv("USER")
			}
			if val != "" {
				args = append(args, "--setenv", pattern, val)
				if verbose {
					fmt.Fprintf(os.Stderr, "[verbose]   --setenv %s=%s (passed)\n", pattern, val)
				}
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

	if config.FeatureEnabledDefault(cfg, func(f *config.FeaturesConfig) *bool { return f.EnableDBus }, false) {
		addDBusArgs(&args, cfg, dryRun)
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
	args = append(args, "--ro-bind-try", "/run/systemd/resolve", "/run/systemd/resolve")
	args = append(args, "--ro-bind-try", "/opt/google", "/opt/google")
	args = append(args, "--ro-bind-try", "/etc/ssl", "/etc/ssl")
	args = append(args, "--ro-bind-try", "/etc/ca-certificates", "/etc/ca-certificates")
	args = append(args, "--ro-bind-try", "/usr/share/ca-certificates", "/usr/share/ca-certificates")

	resolvPath := filepath.Join(sandboxDir, "etc", "resolv.conf")
	if _, err := os.Stat(resolvPath); err == nil {
		args = append(args, "--ro-bind", resolvPath, "/etc/resolv.conf")
	} else if _, err := os.Stat("/run/systemd/resolve/resolv.conf"); err == nil {
		args = append(args, "--ro-bind", "/run/systemd/resolve/resolv.conf", "/etc/resolv.conf")
	} else if _, err := os.Stat("/etc/resolv.conf"); err == nil {
		args = append(args, "--ro-bind", "/etc/resolv.conf", "/etc/resolv.conf")
	}

	gaiPath := filepath.Join(sandboxDir, "etc", "gai.conf")
	if _, err := os.Stat(gaiPath); err == nil {
		args = append(args, "--ro-bind", gaiPath, "/etc/gai.conf")
	}

	hostsPath := filepath.Join(sandboxDir, "etc", "hosts")
	if _, err := os.Stat(hostsPath); err == nil {
		args = append(args, "--ro-bind", hostsPath, "/etc/hosts")
	}

	addMaskArgs(&args, cfg, homeDir, currentDir, verbose)

	return args
}
