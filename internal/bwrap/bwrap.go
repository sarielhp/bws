package bwrap

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"bw/internal/config"
	"bw/internal/ssh"
	"bw/internal/util"
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

	if cfg.Env != nil {
		for k, defaultVal := range cfg.Env {
			if k == "PATH" {
				continue
			}
			val := os.Getenv(k)
			if val == "" {
				val = defaultVal
			}
			if k == "LC_ALL" && os.Getenv("LC_ALL") == "" {
				val = os.Getenv("LANG")
				if val == "" {
					val = defaultVal
				}
			}
			if k == "LOGNAME" && os.Getenv("LOGNAME") == "" {
				val = os.Getenv("USER")
				if val == "" {
					val = defaultVal
				}
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
			if strings.HasPrefix(p, "~") {
				resolved = append(resolved, util.ExpandHome(p))
			} else {
				resolved = append(resolved, p)
			}
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

	hostTmp := "/tmp/bw/SANDBOX_TMP"
	if !dryRun {
		os.MkdirAll("/tmp/bw", 0755)
		var err error
		hostTmp, err = os.MkdirTemp("/tmp/bw", "sandbox_")
		if err != nil {
			hostTmp = "/tmp/bw/SANDBOX_TMP"
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

	return args
}

func addSSHArgs(cfg *config.Config, sandboxDir string, args *[]string, dryRun bool) {
	sshKeys := cfg.Features.SSHKeys
	var sshAuthSock string

	if dryRun {
		sock := os.Getenv("SSH_AUTH_SOCK")
		if sock != "" {
			if fi, err := os.Stat(sock); err == nil && fi.Mode()&os.ModeSocket != 0 {
				sshAuthSock = sock
			}
		}
	} else {
		if (len(sshKeys) == 0) && config.GetBool(cfg, func(c *config.Config) *bool {
			if c.Features == nil {
				return nil
			}
			return c.Features.AutoRepoDeployKey
		}, true) && util.CommandExists("ssh-agent") && util.CommandExists("ssh-add") {
			if key := ssh.GetAutoDeployKey(); key != "" {
				sshKeys = []string{key}
			}
		}
		sshAuthSock = ssh.EnsureAgent(sshKeys)
	}

	hostSSHDir := filepath.Join(util.HomeDir(), ".ssh")

	if sshAuthSock != "" {
		if fi, err := os.Stat(sshAuthSock); err == nil && fi.Mode()&os.ModeSocket != 0 {
			*args = append(*args, "--bind", sshAuthSock, sshAuthSock)
			*args = append(*args, "--setenv", "SSH_AUTH_SOCK", sshAuthSock)

			if fi, err := os.Stat(filepath.Join(hostSSHDir, "config")); err == nil && !fi.IsDir() {
				*args = append(*args, "--ro-bind", filepath.Join(hostSSHDir, "config"),
					filepath.Join(util.HomeDir(), ".ssh", "config"))
			}
			if fi, err := os.Stat(filepath.Join(hostSSHDir, "known_hosts")); err == nil && !fi.IsDir() {
				*args = append(*args, "--bind", filepath.Join(hostSSHDir, "known_hosts"),
					filepath.Join(util.HomeDir(), ".ssh", "known_hosts"))
			}
		}
	} else if fi, err := os.Stat(hostSSHDir); err == nil && fi.IsDir() {
		if !dryRun {
			os.MkdirAll(filepath.Join(sandboxDir, ".ssh"), 0755)
		}
		for _, f := range []string{"config", "known_hosts"} {
			hostF := filepath.Join(hostSSHDir, f)
			if fi, err := os.Stat(hostF); err == nil && !fi.IsDir() {
				alreadyBound := false
				for _, a := range *args {
					if a == hostF {
						alreadyBound = true
						break
					}
				}
				if !alreadyBound {
					*args = append(*args, "--ro-bind", hostF, filepath.Join(util.HomeDir(), ".ssh", f))
				}
			}
		}
	}
}

func addX11Args(args *[]string) {
	display := os.Getenv("DISPLAY")
	if display == "" {
		return
	}
	*args = append(*args, "--setenv", "DISPLAY", display)

	xauth := os.Getenv("XAUTHORITY")
	if xauth == "" {
		xauth = filepath.Join(util.HomeDir(), ".Xauthority")
	}
	if fi, err := os.Stat(xauth); err == nil && !fi.IsDir() {
		*args = append(*args, "--ro-bind", xauth, filepath.Join(util.HomeDir(), ".Xauthority"))
		*args = append(*args, "--setenv", "XAUTHORITY", filepath.Join(util.HomeDir(), ".Xauthority"))
	}

	uid := os.Getuid()
	userRunDir := os.Getenv("XDG_RUNTIME_DIR")
	if userRunDir == "" {
		userRunDir = fmt.Sprintf("/run/user/%d", uid)
	}
	if fi, err := os.Stat(userRunDir); err == nil && fi.IsDir() {
		*args = append(*args, "--bind-try", userRunDir, userRunDir)
		*args = append(*args, "--setenv", "XDG_RUNTIME_DIR", userRunDir)
	}

	*args = append(*args, "--setenv", "NO_AT_SPI", "1")
}

func addWSLArgs(args *[]string) {
	wslInterop := os.Getenv("WSL_INTEROP")
	isWSL := (wslInterop != "") || dirExists("/run/WSL") || fileExists("/proc/sys/fs/binfmt_misc/WSLInterop")

	if isWSL {
		*args = append(*args, "--ro-bind-try", "/init", "/init")
		*args = append(*args, "--bind-try", "/run/WSL", "/run/WSL")
		if wslInterop != "" {
			*args = append(*args, "--setenv", "WSL_INTEROP", wslInterop)
		}
		if fileExists("/mnt/c/Windows/System32/clip.exe") {
			*args = append(*args, "--ro-bind-try", "/mnt/c/Windows/System32/clip.exe", "/mnt/c/Windows/System32/clip.exe")
		}
	}
}

func addEtcAutoBindArgs(args *[]string) {
	entries, err := os.ReadDir("/etc")
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if name != "resolv.conf" {
			path := filepath.Join("/etc", name)
			*args = append(*args, "--ro-bind-try", path, path)
		}
	}
}

func addOptBind(args *[]string) {
	if fi, err := os.Stat("/opt"); err == nil && fi.IsDir() {
		alreadyBound := false
		for _, a := range *args {
			if a == "/opt" {
				alreadyBound = true
				break
			}
		}
		if !alreadyBound {
			*args = append(*args, "--ro-bind-try", "/opt", "/opt")
			if realPath, err := filepath.EvalSymlinks("/opt"); err == nil && realPath != "/opt" {
				if fi, err := os.Stat(realPath); err == nil && fi.IsDir() {
					alreadyBound = false
					for _, a := range *args {
						if a == realPath {
							alreadyBound = true
							break
						}
					}
					if !alreadyBound {
						*args = append(*args, "--ro-bind-try", realPath, realPath)
					}
				}
			}
		}
	}
}

func addQuartoBind(args *[]string) {
	if !util.CommandExists("quarto") {
		return
	}
	quartoPath, err := execLookPath("quarto")
	if err != nil {
		return
	}
	realPath, err := filepath.EvalSymlinks(quartoPath)
	if err != nil {
		realPath = quartoPath
	}
	binDir := filepath.Dir(realPath)
	rootDir := filepath.Dir(binDir)

	if fi, err := os.Stat(rootDir); err == nil && fi.IsDir() {
		alreadyBound := false
		for _, a := range *args {
			if a == rootDir {
				alreadyBound = true
				break
			}
		}
		if !alreadyBound {
			*args = append(*args, "--ro-bind-try", rootDir, rootDir)
		}
	}
	if fi, err := os.Stat(binDir); err == nil && fi.IsDir() {
		alreadyBound := false
		for _, a := range *args {
			if a == binDir {
				alreadyBound = true
				break
			}
		}
		if !alreadyBound {
			*args = append(*args, "--ro-bind-try", binDir, binDir)
		}
	}
}

func dirExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

func execLookPath(file string) (string, error) {
	if strings.Contains(file, "/") {
		if fi, err := os.Stat(file); err == nil && !fi.IsDir() && fi.Mode()&0111 != 0 {
			return file, nil
		}
		return "", fmt.Errorf("not found")
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		p := filepath.Join(dir, file)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() && fi.Mode()&0111 != 0 {
			return p, nil
		}
	}
	return "", fmt.Errorf("not found")
}
