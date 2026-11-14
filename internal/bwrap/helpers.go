package bwrap

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"bws/internal/config"
	"bws/internal/dbus"
	"bws/internal/ssh"
	"bws/internal/util"
)

func addSSHArgs(cfg *config.Config, sandboxDir, currentDir string, args *[]string, dryRun bool) {
	sshAuthSock := ""
	if dryRun {
		if len(featureSSHKeys(cfg)) > 0 || autoDeployKeyEnabled(cfg) {
			sshAuthSock = "<session-ssh-agent>"
		}
	} else {
		sshAuthSock = ssh.StartSessionAgent(sessionSSHKeys(cfg, currentDir))
	}
	if sshAuthSock != "" {
		*args = append(*args, "--bind", sshAuthSock, sshAuthSock, "--setenv", "SSH_AUTH_SOCK", sshAuthSock)
	}

	hostSSHDir := filepath.Join(util.HomeDir(), ".ssh")
	if !dirExists(hostSSHDir) {
		return
	}
	if !dryRun && sshAuthSock == "" {
		os.MkdirAll(filepath.Join(sandboxDir, ".ssh"), 0755)
	}
	for _, f := range []string{"config", "known_hosts"} {
		hostF := filepath.Join(hostSSHDir, f)
		if fileExists(hostF) && !isAlreadyBound(*args, hostF) {
			*args = append(*args, "--ro-bind", hostF, filepath.Join(util.HomeDir(), ".ssh", f))
		}
	}
}

func featureSSHKeys(cfg *config.Config) []string {
	if cfg == nil || cfg.Features == nil {
		return nil
	}
	return cfg.Features.SSHKeys
}

func autoDeployKeyEnabled(cfg *config.Config) bool {
	return config.FeatureEnabled(cfg, func(f *config.FeaturesConfig) *bool { return f.AutoRepoDeployKey })
}

// sessionSSHKeys returns the keys to load into the session agent: the
// configured ssh_keys, or else the deploy key of the repository bound to the
// workspace. It never falls back to the default identities in ~/.ssh.
func sessionSSHKeys(cfg *config.Config, currentDir string) []string {
	if keys := featureSSHKeys(cfg); len(keys) > 0 {
		return keys
	}
	if !autoDeployKeyEnabled(cfg) || !util.CommandExists("git") || !util.CommandExists("gh") {
		return nil
	}
	pinned := ""
	if cfg.Features != nil {
		pinned = cfg.Features.DeployKeyRepo
	}
	repo, err := ssh.ResolveDeployRepo(pinned, currentDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[bws] Warning: no deploy key loaded: %v\n", err)
		return nil
	}
	if repo == "" {
		return nil
	}
	if key := ssh.GetAutoDeployKey(repo); key != "" {
		return []string{key}
	}
	return nil
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

	*args = append(*args, "--setenv", "NO_AT_SPI", "1")
}

func addDBusArgs(args *[]string, cfg *config.Config, dryRun bool) {
	if !config.FeatureEnabledDefault(cfg, func(f *config.FeaturesConfig) *bool { return f.EnableDBus }, false) {
		return
	}

	hostSock := dbus.HostSessionBusSocketPath()
	if hostSock == "" {
		return
	}

	destPath, destDir := dbus.SandboxDestinationPaths()

	if dryRun {
		*args = append(*args, "--bind", "<filtered-dbus-proxy>", destPath)
		*args = append(*args, "--setenv", "DBUS_SESSION_BUS_ADDRESS", fmt.Sprintf("unix:path=%s", destPath))
		*args = append(*args, "--setenv", "XDG_RUNTIME_DIR", destDir)
		return
	}

	// In non-dryRun, if AllowRawDBus is explicitly enabled and xdg-dbus-proxy is missing, raw socket is bound
	if config.FeatureEnabledDefault(cfg, func(f *config.FeaturesConfig) *bool { return f.AllowRawDBus }, false) && !util.CommandExists("xdg-dbus-proxy") {
		if fi, err := os.Stat(hostSock); err == nil && (fi.Mode()&os.ModeSocket != 0 || !fi.IsDir()) {
			*args = append(*args, "--bind", hostSock, destPath)
			*args = append(*args, "--setenv", "DBUS_SESSION_BUS_ADDRESS", fmt.Sprintf("unix:path=%s", destPath))
			*args = append(*args, "--setenv", "XDG_RUNTIME_DIR", destDir)
		}
	}
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
		if name != "resolv.conf" && name != "gai.conf" && name != "hosts" {
			path := filepath.Join("/etc", name)
			*args = append(*args, "--ro-bind-try", path, path)
		}
	}
}

func isAlreadyBound(args []string, target string) bool {
	for _, a := range args {
		if a == target {
			return true
		}
	}
	return false
}

func isSystemOrHomeRoot(p string) bool {
	clean := filepath.Clean(p)
	if clean == "/" || clean == "." || clean == "/usr" || clean == "/bin" || clean == "/sbin" ||
		clean == "/usr/local" || clean == "/usr/bin" || clean == "/usr/sbin" ||
		clean == "/etc" || clean == "/var" || clean == "/opt" || clean == "/home" {
		return true
	}
	home := filepath.Clean(util.HomeDir())
	if clean == home || clean == filepath.Join(home, ".local") ||
		clean == filepath.Join(home, ".local", "bin") || clean == filepath.Join(home, "bin") {
		return true
	}
	return false
}

func addOptBind(args *[]string) {
	if fi, err := os.Stat("/opt"); err == nil && fi.IsDir() {
		if !isAlreadyBound(*args, "/opt") {
			*args = append(*args, "--ro-bind-try", "/opt", "/opt")
			if realPath, err := filepath.EvalSymlinks("/opt"); err == nil && realPath != "/opt" {
				if fi, err := os.Stat(realPath); err == nil && fi.IsDir() && !isAlreadyBound(*args, realPath) {
					*args = append(*args, "--ro-bind-try", realPath, realPath)
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

	if !isSystemOrHomeRoot(rootDir) {
		if fi, err := os.Stat(rootDir); err == nil && fi.IsDir() && !isAlreadyBound(*args, rootDir) {
			*args = append(*args, "--ro-bind-try", rootDir, rootDir)
		}
	}
	if !isSystemOrHomeRoot(binDir) {
		if fi, err := os.Stat(binDir); err == nil && fi.IsDir() && !isAlreadyBound(*args, binDir) {
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

// forgeTools maps each forge CLI to the binaries and config paths masked
// while forge blocking is on.
func forgeTools(homeDir string) map[string][]string {
	bins := func(name string, extra ...string) []string {
		out := []string{"/usr/bin/" + name, "/usr/local/bin/" + name, "/bin/" + name}
		if p, err := execLookPath(name); err == nil && p != "" {
			out = append([]string{p}, out...)
		}
		return append(out, extra...)
	}
	home := func(parts ...string) string { return filepath.Join(append([]string{homeDir}, parts...)...) }
	return map[string][]string{
		"gh": bins("gh", home(".local", "bin", "gh"), home("bin", "gh"),
			home(".config", "gh"), home(".local", "share", "gh"), home(".local", "state", "gh")),
		"glab": bins("glab", home(".local", "bin", "glab"), home("bin", "glab"),
			home(".config", "glab-cli"), home(".local", "share", "glab-cli"), home(".local", "state", "glab-cli")),
		"hub": bins("hub", home(".config", "hub")),
		"tea": bins("tea", home(".config", "tea")),
	}
}

// collectForgeMasks returns the forge paths to mask. A forge CLI whose
// profile was selected is left usable unless block_gh was set explicitly;
// credential stores stay masked either way.
func collectForgeMasks(cfg *config.Config, homeDir string) []string {
	explicit := cfg.Features != nil && cfg.Features.BlockGH != nil
	tools := forgeTools(homeDir)
	var masks []string
	for _, name := range []string{"gh", "glab", "hub", "tea"} {
		if !explicit && slices.Contains(cfg.ActiveProfiles, name) {
			continue
		}
		masks = append(masks, tools[name]...)
	}
	return append(masks,
		filepath.Join(homeDir, ".git-credentials"),
		filepath.Join(homeDir, ".config", "git", "credentials"),
		filepath.Join(homeDir, ".netrc"),
		filepath.Join(homeDir, ".config", "netrc"),
	)
}

func addMaskArgs(args *[]string, cfg *config.Config, homeDir, currentDir string, verbose bool) {
	maskList := append([]string{}, cfg.Mask...)
	if config.HistoryMaskEnabled(cfg) {
		maskList = append(maskList, config.DefaultHistoryMasks...)
	}
	if config.BlockGH(cfg) {
		maskList = append(maskList, collectForgeMasks(cfg, homeDir)...)
	}

	seen := make(map[string]bool)
	for _, maskPath := range maskList {
		expanded := resolveMaskPath(maskPath, homeDir, currentDir)
		if expanded == "" || seen[expanded] {
			continue
		}
		seen[expanded] = true

		if fi, err := os.Stat(expanded); err == nil {
			if fi.IsDir() {
				*args = append(*args, "--tmpfs", expanded)
				if verbose {
					fmt.Fprintf(os.Stderr, "[verbose]   --tmpfs %s (masked directory)\n", expanded)
				}
			} else {
				*args = append(*args, "--ro-bind-try", "/dev/null", expanded)
				if verbose {
					fmt.Fprintf(os.Stderr, "[verbose]   --ro-bind-try /dev/null %s (masked file)\n", expanded)
				}
			}
		}
	}

	deployKeysDir := filepath.Join(homeDir, ".sandbox", "deploy_keys")
	if fi, err := os.Stat(deployKeysDir); err == nil && fi.IsDir() && !seen[deployKeysDir] {
		seen[deployKeysDir] = true
		*args = append(*args, "--tmpfs", deployKeysDir)
		if verbose {
			fmt.Fprintf(os.Stderr, "[verbose]   --tmpfs %s (masked private deploy keys dir)\n", deployKeysDir)
		}
	}

	wsRoot, _ := config.FindWorkspaceRoot(currentDir)
	*args = append(*args, "--tmpfs", config.TrustDir())
	for _, dir := range []string{currentDir, wsRoot} {
		bwsDir := filepath.Join(dir, ".bws")
		if fi, err := os.Stat(bwsDir); err == nil && fi.IsDir() && !seen[bwsDir] {
			seen[bwsDir] = true
			*args = append(*args, "--tmpfs", bwsDir)
			if verbose {
				fmt.Fprintf(os.Stderr, "[verbose]   --tmpfs %s (masked workspace .bws config dir)\n", bwsDir)
			}
		}
		bwsFile := filepath.Join(dir, ".bws.jsonc")
		if fi, err := os.Stat(bwsFile); err == nil && !fi.IsDir() && !seen[bwsFile] {
			seen[bwsFile] = true
			*args = append(*args, "--ro-bind-try", "/dev/null", bwsFile)
			if verbose {
				fmt.Fprintf(os.Stderr, "[verbose]   --ro-bind-try /dev/null %s (masked workspace .bws.jsonc config file)\n", bwsFile)
			}
		}
	}
}

// resolveMaskPath resolves a mask entry like a bind path and returns "" for
// anything that is still not absolute, so it can never reach bwrap as a flag.
func resolveMaskPath(p, homeDir, currentDir string) string {
	resolved := resolveBindPath(p, homeDir, currentDir)
	if !filepath.IsAbs(resolved) {
		return ""
	}
	return resolved
}
