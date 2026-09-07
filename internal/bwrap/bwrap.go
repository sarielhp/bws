package bwrap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bws/internal/config"
	"bws/internal/util"
)

var blockedForgeTokens = []string{
	"GH_TOKEN",
	"GITHUB_TOKEN",
	"GH_ENTERPRISE_TOKEN",
	"GITHUB_ENTERPRISE_TOKEN",
}

func isBlockedForgeToken(key string) bool {
	for _, t := range blockedForgeTokens {
		if key == t {
			return true
		}
	}
	return false
}

func isExplicitInEnv(cfg *config.Config, key string) bool {
	if cfg == nil || cfg.Env == nil {
		return false
	}
	_, ok := cfg.Env[key]
	return ok
}

func addSystemAndNetArgs(cfg *config.Config, args *[]string, verbose bool) {
	*args = append(*args, "--tmpfs", "/etc", "--unshare-ipc", "--unshare-pid", "--new-session")
	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose]   --tmpfs /etc\n")
		fmt.Fprintf(os.Stderr, "[verbose]   --unshare-ipc\n")
		fmt.Fprintf(os.Stderr, "[verbose]   --unshare-pid\n")
		fmt.Fprintf(os.Stderr, "[verbose]   --new-session\n")
	}

	if config.FeatureEnabledDefault(cfg, func(f *config.FeaturesConfig) *bool { return f.NoNet }, false) ||
		config.FeatureEnabledDefault(cfg, func(f *config.FeaturesConfig) *bool { return f.UnshareNet }, false) {
		*args = append(*args, "--unshare-net")
		if verbose {
			fmt.Fprintf(os.Stderr, "[verbose]   --unshare-net (air-gapped network namespace)\n")
		}
	} else {
		*args = append(*args, "--share-net")
		if verbose {
			fmt.Fprintf(os.Stderr, "[verbose]   --share-net\n")
		}
	}

	if cfg.System != nil {
		if config.GetBool(cfg, func(c *config.Config) *bool { return c.System.Clearenv }, false) {
			*args = append(*args, "--clearenv")
			if verbose {
				fmt.Fprintf(os.Stderr, "[verbose]   --clearenv\n")
			}
		}
		if config.GetBool(cfg, func(c *config.Config) *bool { return c.System.UnshareUTS }, false) {
			*args = append(*args, "--unshare-uts")
			if verbose {
				fmt.Fprintf(os.Stderr, "[verbose]   --unshare-uts\n")
			}
		}
		if cfg.System.Hostname != nil {
			*args = append(*args, "--hostname", *cfg.System.Hostname)
			if verbose {
				fmt.Fprintf(os.Stderr, "[verbose]   --hostname %s\n", *cfg.System.Hostname)
			}
		}
	}
}

func addSandboxHomeBind(sandboxDir, homeDir string, args *[]string, dryRun, verbose bool) {
	if sandboxDir == "" {
		return
	}
	if dryRun && strings.HasPrefix(sandboxDir, "<") {
		*args = append(*args, "--bind", sandboxDir, homeDir)
	} else if _, err := os.Stat(sandboxDir); err == nil {
		*args = append(*args, "--bind", sandboxDir, homeDir)
		if verbose {
			fmt.Fprintf(os.Stderr, "[verbose]   --bind %s %s (sandbox home)\n", sandboxDir, homeDir)
		}
	}
}

func addPassEnvArgs(cfg *config.Config, args *[]string, verbose bool) {
	for _, pattern := range cfg.PassEnv {
		if strings.HasSuffix(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, "*")
			for _, envItem := range os.Environ() {
				parts := strings.SplitN(envItem, "=", 2)
				if len(parts) == 2 && strings.HasPrefix(parts[0], prefix) {
					if isBlockedForgeToken(parts[0]) && !isExplicitInEnv(cfg, parts[0]) {
						continue
					}
					*args = append(*args, "--setenv", parts[0], parts[1])
					if verbose {
						fmt.Fprintf(os.Stderr, "[verbose]   --setenv %s=%s (passed wildcard)\n", parts[0], parts[1])
					}
				}
			}
		} else {
			if pattern == "PATH" {
				continue
			}
			if isBlockedForgeToken(pattern) && !isExplicitInEnv(cfg, pattern) {
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
				*args = append(*args, "--setenv", pattern, val)
				if verbose {
					fmt.Fprintf(os.Stderr, "[verbose]   --setenv %s=%s (passed)\n", pattern, val)
				}
			}
		}
	}

	clearenv := config.GetBool(cfg, func(c *config.Config) *bool {
		if c.System == nil {
			return nil
		}
		return c.System.Clearenv
	}, false)
	if !clearenv {
		for _, token := range blockedForgeTokens {
			if !isExplicitInEnv(cfg, token) {
				*args = append(*args, "--unsetenv", token)
			}
		}
	}
}

func addCustomEnvArgs(cfg *config.Config, homeDir string, args *[]string, verbose bool) {
	if cfg.Env == nil {
		return
	}
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
			*args = append(*args, "--setenv", k, val)
			if verbose {
				fmt.Fprintf(os.Stderr, "[verbose]   --setenv %s=%s\n", k, val)
			}
		}
	}
}

func addPathArgs(cfg *config.Config, homeDir string, args *[]string, verbose bool) {
	if len(cfg.Path) == 0 {
		return
	}
	resolved := make([]string, 0, len(cfg.Path))
	for _, p := range cfg.Path {
		resolvedP := strings.ReplaceAll(p, config.HomeToken, homeDir)
		resolvedP = util.ExpandHome(resolvedP)
		resolved = append(resolved, resolvedP)
	}
	pathVal := strings.Join(resolved, ":")
	*args = append(*args, "--setenv", "PATH", pathVal)
	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose]   --setenv PATH=%s\n", pathVal)
	}
}

func addFeatureMountArgs(cfg *config.Config, sandboxDir string, args *[]string, dryRun bool) {
	if config.FeatureEnabled(cfg, func(f *config.FeaturesConfig) *bool { return f.EnableSSH }) {
		addSSHArgs(cfg, sandboxDir, args, dryRun)
	}
	if config.FeatureEnabled(cfg, func(f *config.FeaturesConfig) *bool { return f.EnableX11 }) {
		addX11Args(args)
	}
	if config.FeatureEnabledDefault(cfg, func(f *config.FeaturesConfig) *bool { return f.EnableDBus }, false) {
		addDBusArgs(args, cfg, dryRun)
	}
	if config.GetBool(cfg, func(c *config.Config) *bool {
		if c.Features == nil {
			return nil
		}
		return c.Features.EnableWSL
	}, true) {
		addWSLArgs(args)
	}
	if config.FeatureEnabled(cfg, func(f *config.FeaturesConfig) *bool { return f.EnableEtcAutoBind }) {
		addEtcAutoBindArgs(args)
	}
	addOptBind(args)
	addQuartoBind(args)
}

func addStandardMounts(cfg *config.Config, sandboxDir, currentDir string, args *[]string, dryRun, verbose bool) {
	hostTmp := "/tmp/bws/SANDBOX_TMP"
	if !dryRun {
		os.MkdirAll("/tmp/bws", 0755)
		if tmp, err := os.MkdirTemp("/tmp/bws", "sandbox_"); err == nil {
			hostTmp = tmp
		}
	}

	*args = append(*args,
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

	if config.FeatureEnabled(cfg, func(f *config.FeaturesConfig) *bool { return f.EnableX11 }) && os.Getenv("DISPLAY") != "" {
		*args = append(*args, "--ro-bind-try", "/tmp/.X11-unix", "/tmp/.X11-unix")
	}

	for _, p := range []string{
		"/run/systemd/resolve",
		"/opt/google",
		"/etc/ssl",
		"/etc/ca-certificates",
		"/usr/share/ca-certificates",
	} {
		*args = append(*args, "--ro-bind-try", p, p)
	}

	resolvPath := filepath.Join(sandboxDir, "etc", "resolv.conf")
	if _, err := os.Stat(resolvPath); err == nil {
		*args = append(*args, "--ro-bind", resolvPath, "/etc/resolv.conf")
	} else if _, err := os.Stat("/run/systemd/resolve/resolv.conf"); err == nil {
		*args = append(*args, "--ro-bind", "/run/systemd/resolve/resolv.conf", "/etc/resolv.conf")
	} else if _, err := os.Stat("/etc/resolv.conf"); err == nil {
		*args = append(*args, "--ro-bind", "/etc/resolv.conf", "/etc/resolv.conf")
	}

	gaiPath := filepath.Join(sandboxDir, "etc", "gai.conf")
	if _, err := os.Stat(gaiPath); err == nil {
		*args = append(*args, "--ro-bind", gaiPath, "/etc/gai.conf")
	}

	hostsPath := filepath.Join(sandboxDir, "etc", "hosts")
	if _, err := os.Stat(hostsPath); err == nil {
		*args = append(*args, "--ro-bind", hostsPath, "/etc/hosts")
	}
}

func BuildArgs(cfg *config.Config, sandboxDir, currentDir string, dryRun, verbose bool) []string {
	var args []string
	homeDir := util.HomeDir()

	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] Building bwrap argument list...\n")
	}

	addSystemAndNetArgs(cfg, &args, verbose)
	addSandboxHomeBind(sandboxDir, homeDir, &args, dryRun, verbose)
	addStandardMounts(cfg, sandboxDir, currentDir, &args, dryRun, verbose)
	addFeatureMountArgs(cfg, sandboxDir, &args, dryRun)
	args = append(args, buildBinds(cfg, sandboxDir, homeDir, currentDir, verbose)...)

	addPassEnvArgs(cfg, &args, verbose)
	addCustomEnvArgs(cfg, homeDir, &args, verbose)
	addPathArgs(cfg, homeDir, &args, verbose)

	addMaskArgs(&args, cfg, homeDir, currentDir, verbose)

	return args
}
