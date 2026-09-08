package cli

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"bws/internal/config"
	"bws/internal/profile"
	"bws/internal/util"

	"github.com/fatih/color"
)

// CheckStatus represents the outcome status of an individual diagnostic check.
type CheckStatus int

const (
	StatusPass CheckStatus = iota
	StatusWarn
	StatusFail
)

// CheckResult records the detailed findings and recommendations of a diagnostic check.
type CheckResult struct {
	Name    string
	Status  CheckStatus
	Message string
	Details []string
	Advice  []string
}

// DoctorColors provides styled printer functions with terminal ANSI support.
type DoctorColors struct {
	OK     func(a ...interface{}) string
	Warn   func(a ...interface{}) string
	Fail   func(a ...interface{}) string
	Header func(a ...interface{}) string
	Path   func(a ...interface{}) string
	Dim    func(a ...interface{}) string
}

func initDoctorColors(enableColor bool) DoctorColors {
	if !enableColor {
		plain := fmt.Sprint
		return DoctorColors{
			OK:     plain,
			Warn:   plain,
			Fail:   plain,
			Header: plain,
			Path:   plain,
			Dim:    plain,
		}
	}
	newColor := func(attrs ...color.Attribute) func(a ...interface{}) string {
		c := color.New(attrs...)
		c.EnableColor()
		return c.SprintFunc()
	}
	return DoctorColors{
		OK:     newColor(color.FgGreen, color.Bold),
		Warn:   newColor(color.FgYellow, color.Bold),
		Fail:   newColor(color.FgRed, color.Bold),
		Header: newColor(color.FgCyan, color.Bold),
		Path:   newColor(color.FgCyan),
		Dim:    newColor(color.FgHiBlack),
	}
}

func shouldUseColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return IsInteractiveTTY(int(os.Stdout.Fd()))
}

// UsrnsProber defines a function signature for testing unprivileged user namespace setup.
type UsrnsProber func() (string, error)

// DefaultUsrnsProber runs `bwrap --ro-bind / / /bin/true` to probe unprivileged namespace support.
func DefaultUsrnsProber() (string, error) {
	bwrapPath, err := exec.LookPath("bwrap")
	if err != nil {
		return "", fmt.Errorf("bubblewrap executable ('bwrap') not found in PATH")
	}
	trueBin := "/bin/true"
	if _, err := os.Stat(trueBin); err != nil {
		if _, err2 := os.Stat("/usr/bin/true"); err2 == nil {
			trueBin = "/usr/bin/true"
		} else {
			trueBin = "true"
		}
	}
	cmd := exec.Command(bwrapPath, "--ro-bind", "/", "/", trueBin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(out)), err
	}
	return "", nil
}

// CheckKernelUserns probes unprivileged namespace creation and returns sysctl guidance on failure.
func CheckKernelUserns(prober UsrnsProber) CheckResult {
	res := CheckResult{Name: "Kernel & Bubblewrap User Namespaces"}
	if prober == nil {
		prober = DefaultUsrnsProber
	}
	out, err := prober()
	if err != nil {
		res.Status = StatusFail
		res.Message = "Bubblewrap cannot create unprivileged user namespaces."
		if out != "" {
			res.Details = append(res.Details, fmt.Sprintf("Probe error: %s (%v)", out, err))
		} else {
			res.Details = append(res.Details, fmt.Sprintf("Probe error: %v", err))
		}
		res.Advice = append(res.Advice,
			"Enable unprivileged user namespaces: `sudo sysctl -w kernel.unprivileged_userns_clone=1`",
			"On Ubuntu 24.04+ (AppArmor restriction): `sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0`",
			"Or create an AppArmor profile allowing userns for /usr/bin/bwrap in /etc/apparmor.d/bwrap.",
		)
		return res
	}
	res.Status = StatusPass
	res.Message = "Unprivileged user namespace creation succeeded."
	return res
}

// ToolLookup checks if a command binary is available in the host environment.
type ToolLookup func(name string) bool

// CheckCriticalTools verifies that bwrap, tmux, shell, and git are installed and executable.
func CheckCriticalTools(lookup ToolLookup) CheckResult {
	res := CheckResult{Name: "Critical Host Tools"}
	if lookup == nil {
		lookup = util.CommandExists
	}
	var missing []string
	for _, t := range []string{"bwrap", "tmux", "git"} {
		if !lookup(t) {
			missing = append(missing, t)
		}
	}

	sh := os.Getenv("SHELL")
	shellFound := false
	if sh != "" {
		if filepath.IsAbs(sh) {
			if fi, err := os.Stat(sh); err == nil && fi.Mode()&0111 != 0 {
				shellFound = true
			}
		} else if lookup(sh) {
			shellFound = true
		}
	}
	if !shellFound {
		if lookup("bash") {
			sh = "bash"
			shellFound = true
		} else if lookup("sh") {
			sh = "sh"
			shellFound = true
		}
	}
	if !shellFound {
		missing = append(missing, "shell (bash/sh)")
	}

	if len(missing) > 0 {
		res.Status = StatusFail
		res.Message = fmt.Sprintf("Missing critical host tool(s): %s", strings.Join(missing, ", "))
		res.Advice = append(res.Advice, "Install missing host prerequisites (e.g. `sudo apt install bubblewrap tmux git`).")
		return res
	}

	res.Status = StatusPass
	res.Message = fmt.Sprintf("All critical host tools found: bwrap, tmux, git, shell (%s).", sh)
	return res
}

func collectDeclaredProfiles(globalCfg, localCfg *config.Config) []string {
	seen := make(map[string]bool)
	var list []string
	add := func(profiles []string) {
		for _, p := range profiles {
			p = strings.TrimSpace(p)
			if p != "" && !seen[p] {
				seen[p] = true
				list = append(list, p)
			}
		}
	}
	if globalCfg != nil {
		add(globalCfg.Profiles)
	}
	if localCfg != nil {
		add(localCfg.Profiles)
	}
	return list
}

// CheckConfigFilesAndProfiles validates global/local JSONC syntax and declared profile catalog presence.
func CheckConfigFilesAndProfiles(cwd string) CheckResult {
	res := CheckResult{Name: "Configuration Files & Profiles"}
	globalPath := config.GlobalPath()
	var globalCfg, localCfg *config.Config
	var loadErrors []string

	if fi, err := os.Stat(globalPath); err == nil && !fi.IsDir() {
		if cfg, err := config.LoadFile(globalPath); err != nil {
			loadErrors = append(loadErrors, fmt.Sprintf("Global config syntax error in %s: %v", globalPath, err))
		} else {
			globalCfg = cfg
			res.Details = append(res.Details, fmt.Sprintf("Global config: %s (valid)", globalPath))
		}
	} else {
		res.Details = append(res.Details, fmt.Sprintf("Global config: %s (not found, defaults apply)", globalPath))
	}

	localPath := config.FindLocalPath(cwd)
	if localPath != "" {
		if fi, err := os.Stat(localPath); err == nil && !fi.IsDir() {
			if cfg, err := config.LoadFile(localPath); err != nil {
				loadErrors = append(loadErrors, fmt.Sprintf("Local config syntax error in %s: %v", localPath, err))
			} else {
				localCfg = cfg
				res.Details = append(res.Details, fmt.Sprintf("Local config: %s (valid)", localPath))
			}
		}
	}

	if len(loadErrors) > 0 {
		res.Status = StatusFail
		res.Message = fmt.Sprintf("%d configuration syntax error(s) detected.", len(loadErrors))
		res.Details = append(res.Details, loadErrors...)
		res.Advice = append(res.Advice, "Correct the JSON/JSONC syntax in the affected configuration file.")
		return res
	}

	declared := collectDeclaredProfiles(globalCfg, localCfg)
	registry, err := profile.LoadRegistry(cwd)
	if err != nil {
		res.Status = StatusFail
		res.Message = fmt.Sprintf("Failed to load profile catalog: %v", err)
		return res
	}

	var missingProfiles []string
	for _, p := range declared {
		if _, ok := registry[p]; !ok {
			missingProfiles = append(missingProfiles, p)
		}
	}
	if len(missingProfiles) > 0 {
		res.Status = StatusFail
		res.Message = fmt.Sprintf("Declared profile(s) not found in catalog: %s", strings.Join(missingProfiles, ", "))
		res.Advice = append(res.Advice, "Verify profile names or add missing profiles to ~/.config/bws/profiles/ or .bws/profiles/.")
		return res
	}

	res.Status = StatusPass
	if len(declared) > 0 {
		res.Message = fmt.Sprintf("Configuration syntax valid; all %d declared profile(s) found in catalog.", len(declared))
	} else {
		res.Message = "Configuration syntax valid; no capability profiles declared (default base sandbox)."
	}
	return res
}

func resolveBindHost(p, homeDir, currentDir string) string {
	if p == "" {
		return ""
	}
	p = strings.ReplaceAll(p, config.HomeToken, homeDir)
	p = util.ExpandHome(p)
	if !filepath.IsAbs(p) && currentDir != "" {
		p = filepath.Clean(filepath.Join(currentDir, p))
	}
	return filepath.Clean(p)
}

// CheckDeadBindMounts verifies that configured host directories in binds_rw and binds_ro exist.
func CheckDeadBindMounts(cfg *config.Config, cwd string) CheckResult {
	res := CheckResult{Name: "Stale / Dead Bind Mounts"}
	if cfg == nil {
		res.Status = StatusPass
		res.Message = "No configuration loaded; skipping bind mount verification."
		return res
	}

	homeDir := util.HomeDir()
	var missing []string
	inspect := func(entries []config.BindEntry) {
		for _, b := range entries {
			host := resolveBindHost(b.Host, homeDir, cwd)
			if host == "" {
				continue
			}
			if _, err := os.Stat(host); os.IsNotExist(err) {
				missing = append(missing, b.Host)
			}
		}
	}
	inspect(cfg.BindsRW)
	inspect(cfg.BindsRO)

	if len(missing) > 0 {
		res.Status = StatusWarn
		res.Message = fmt.Sprintf("%d bind mount host path(s) do not exist on host.", len(missing))
		for _, m := range missing {
			res.Details = append(res.Details, fmt.Sprintf("Host path not found: %s", m))
			res.Advice = append(res.Advice, fmt.Sprintf("Run `bws mount rm %s`", m))
		}
		return res
	}

	total := len(cfg.BindsRW) + len(cfg.BindsRO)
	res.Status = StatusPass
	res.Message = fmt.Sprintf("All %d configured bind mount host path(s) exist on host.", total)
	return res
}

type symlinkIssue struct {
	symlink string
	target  string
}

func isPathUnder(target, parent string) bool {
	target = filepath.Clean(target)
	parent = filepath.Clean(parent)
	if target == parent {
		return true
	}
	rel, err := filepath.Rel(parent, target)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..") && rel != ".."
}

func collectMountedRoots(cfg *config.Config, wsRoot, cwd, homeDir string) []string {
	roots := []string{wsRoot, cwd}
	if cfg != nil {
		for _, b := range cfg.BindsRW {
			if h := resolveBindHost(b.Host, homeDir, cwd); h != "" {
				roots = append(roots, h)
			}
		}
		for _, b := range cfg.BindsRO {
			if h := resolveBindHost(b.Host, homeDir, cwd); h != "" {
				roots = append(roots, h)
			}
		}
	}
	// Bubblewrap mounts standard system root partitions read-only.
	systemRoots := []string{
		"/usr", "/lib", "/lib64", "/lib32", "/libx32",
		"/bin", "/sbin", "/etc", "/dev", "/proc", "/sys",
	}
	roots = append(roots, systemRoots...)

	canonical := make([]string, 0, len(roots)*2)
	for _, r := range roots {
		canonical = append(canonical, r)
		if real, err := filepath.EvalSymlinks(r); err == nil && real != r {
			canonical = append(canonical, real)
		}
	}
	return canonical
}

func scanDirSymlinks(dir string, depth, maxDepth int, symlinks *[]string) {
	if depth > maxDepth {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if name == ".git" || name == ".bws" || name == "node_modules" || name == "vendor" {
			continue
		}
		fullPath := filepath.Join(dir, name)
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			*symlinks = append(*symlinks, fullPath)
		} else if e.IsDir() && depth < maxDepth {
			scanDirSymlinks(fullPath, depth+1, maxDepth, symlinks)
		}
	}
}

func findWorkspaceSymlinks(rootDir string, maxDepth int) []string {
	var symlinks []string
	scanDirSymlinks(rootDir, 1, maxDepth, &symlinks)
	return symlinks
}

func auditSymlinks(symlinks []string, mountedRoots []string, cwd string) []symlinkIssue {
	var issues []symlinkIssue
	seen := make(map[string]bool)
	for _, sl := range symlinks {
		target, err := filepath.EvalSymlinks(sl)
		if err != nil {
			rel, _ := os.Readlink(sl)
			if !filepath.IsAbs(rel) {
				rel = filepath.Join(filepath.Dir(sl), rel)
			}
			target = filepath.Clean(rel)
		}
		mounted := false
		for _, mr := range mountedRoots {
			if isPathUnder(target, mr) {
				mounted = true
				break
			}
		}
		if !mounted {
			relSL, err := filepath.Rel(cwd, sl)
			if err != nil {
				relSL = sl
			}
			if !seen[relSL] {
				seen[relSL] = true
				issues = append(issues, symlinkIssue{symlink: relSL, target: target})
			}
		}
	}
	return issues
}

// CheckWorkspaceSymlinks audits symlinks within depth 1-2 for destinations outside sandbox mounts.
func CheckWorkspaceSymlinks(cfg *config.Config, cwd string) CheckResult {
	res := CheckResult{Name: "Workspace Symlink Boundary Audit"}
	wsRoot, _ := config.FindWorkspaceRoot(cwd)
	if wsRoot == "" {
		wsRoot = cwd
	}

	homeDir := util.HomeDir()
	mountedRoots := collectMountedRoots(cfg, wsRoot, cwd, homeDir)
	symlinks := findWorkspaceSymlinks(wsRoot, 2)
	if cwd != wsRoot {
		symlinks = append(symlinks, findWorkspaceSymlinks(cwd, 2)...)
	}

	issues := auditSymlinks(symlinks, mountedRoots, cwd)
	if len(issues) > 0 {
		res.Status = StatusWarn
		res.Message = fmt.Sprintf("%d workspace symlink(s) point outside the mounted sandbox boundary.", len(issues))
		for _, issue := range issues {
			res.Details = append(res.Details, fmt.Sprintf("%s -> %s", issue.symlink, issue.target))
			res.Advice = append(res.Advice, fmt.Sprintf("Run `bws mount add %s`", issue.target))
		}
		return res
	}

	res.Status = StatusPass
	res.Message = "All workspace symlinks point within the mounted sandbox boundary."
	return res
}

func collectEffectiveMasks(cfg *config.Config, cwd, homeDir string) []string {
	var masks []string
	if cfg == nil {
		return masks
	}
	masks = append(masks, cfg.Mask...)
	if config.HistoryMaskEnabled(cfg) {
		masks = append(masks, config.DefaultHistoryMasks...)
	}

	registry, _ := profile.LoadRegistry(cwd)
	mCtx := profile.DetectMatchContext()
	for _, p := range cfg.Profiles {
		if resolved, err := profile.ResolveProfile(p, registry, mCtx); err == nil {
			masks = append(masks, resolved.Mask...)
		}
	}

	wsRoot, _ := config.FindWorkspaceRoot(cwd)
	for _, dir := range []string{cwd, wsRoot} {
		if dir != "" {
			masks = append(masks, filepath.Join(dir, ".bws"))
		}
	}
	return masks
}

// CheckMountMaskingConflicts detects explicit bind mounts shadowed by tmpfs masks.
func CheckMountMaskingConflicts(cfg *config.Config, cwd string) CheckResult {
	res := CheckResult{Name: "Mount Masking Conflicts"}
	if cfg == nil {
		res.Status = StatusPass
		res.Message = "No configuration loaded; skipping mount masking check."
		return res
	}

	homeDir := util.HomeDir()
	masks := collectEffectiveMasks(cfg, cwd, homeDir)
	var conflicts []string

	checkList := func(entries []config.BindEntry) {
		for _, b := range entries {
			dest := b.Sandbox
			if dest == "" {
				dest = b.Host
			}
			destResolved := resolveBindHost(dest, homeDir, cwd)
			for _, m := range masks {
				maskResolved := resolveBindHost(m, homeDir, cwd)
				if destResolved == maskResolved || isPathUnder(destResolved, maskResolved) {
					conflicts = append(conflicts, fmt.Sprintf("Bind target '%s' is shadowed by tmpfs mask '%s'", dest, m))
					break
				}
			}
		}
	}
	checkList(cfg.BindsRW)
	checkList(cfg.BindsRO)

	if len(conflicts) > 0 {
		res.Status = StatusWarn
		res.Message = fmt.Sprintf("%d bind mount(s) shadowed by tmpfs masking rules.", len(conflicts))
		res.Details = append(res.Details, conflicts...)
		res.Advice = append(res.Advice, "Remove shadowed bind mounts via `bws mount rm` or adjust masking profiles.")
		return res
	}

	res.Status = StatusPass
	res.Message = "No explicit bind mounts are shadowed by tmpfs masks."
	return res
}

// DialFunc defines a pluggable network connection dialer for test mocking.
type DialFunc func(network, address string, timeout time.Duration) (net.Conn, error)

// CheckSSHAgent tests SSH agent configuration, socket presence, and responsiveness.
func CheckSSHAgent(cfg *config.Config, dialer DialFunc) CheckResult {
	res := CheckResult{Name: "SSH Agent Socket Health"}
	if dialer == nil {
		dialer = net.DialTimeout
	}
	if !config.FeatureEnabled(cfg, func(f *config.FeaturesConfig) *bool { return f.EnableSSH }) {
		res.Status = StatusPass
		res.Message = "SSH agent forwarding is disabled in configuration."
		return res
	}

	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		res.Status = StatusWarn
		res.Message = "SSH is enabled, but SSH_AUTH_SOCK is not set in host environment."
		res.Advice = append(res.Advice, "Start an agent via `eval $(ssh-agent)` and add keys via `ssh-add`.")
		return res
	}

	fi, err := os.Stat(sock)
	if err != nil || fi.Mode()&os.ModeSocket == 0 {
		res.Status = StatusWarn
		res.Message = fmt.Sprintf("SSH_AUTH_SOCK points to an invalid or missing socket: %s", sock)
		res.Advice = append(res.Advice, "Unset stale SSH_AUTH_SOCK or restart your ssh-agent.")
		return res
	}

	conn, err := dialer("unix", sock, 1*time.Second)
	if err != nil {
		res.Status = StatusWarn
		res.Message = fmt.Sprintf("SSH agent socket at %s is unresponsive: %v", sock, err)
		res.Advice = append(res.Advice, "The ssh-agent may have terminated unexpectedly. Restart your agent.")
		return res
	}
	conn.Close()

	res.Status = StatusPass
	res.Message = fmt.Sprintf("SSH agent socket is active and responsive: %s", sock)
	return res
}

// RunAllDoctorChecks runs all seven diagnostics checks in standard order.
func RunAllDoctorChecks(cfg *config.Config, cwd string, verbose bool) []CheckResult {
	return []CheckResult{
		CheckKernelUserns(nil),
		CheckCriticalTools(nil),
		CheckConfigFilesAndProfiles(cwd),
		CheckDeadBindMounts(cfg, cwd),
		CheckWorkspaceSymlinks(cfg, cwd),
		CheckMountMaskingConflicts(cfg, cwd),
		CheckSSHAgent(cfg, nil),
	}
}

// RenderDoctorReport writes the formatted diagnostic check report to the given writer.
func RenderDoctorReport(w io.Writer, results []CheckResult, colors DoctorColors) (failed, warned, passed int) {
	fmt.Fprintf(w, "%s\n", colors.Header("Bubblewrap Sandbox Diagnostics (bws doctor)"))
	fmt.Fprintf(w, "%s\n\n", colors.Header("==========================================="))

	for _, r := range results {
		var tag string
		switch r.Status {
		case StatusPass:
			tag = colors.OK("[OK]  ")
			passed++
		case StatusWarn:
			tag = colors.Warn("[WARN]")
			warned++
		case StatusFail:
			tag = colors.Fail("[FAIL]")
			failed++
		}

		fmt.Fprintf(w, "%s %s\n", tag, colors.Header(r.Name))
		fmt.Fprintf(w, "       %s\n", r.Message)
		for _, d := range r.Details {
			fmt.Fprintf(w, "       %s\n", colors.Dim("• "+d))
		}
		if len(r.Advice) > 0 {
			fmt.Fprintf(w, "       %s\n", colors.Path("Advice:"))
			for _, a := range r.Advice {
				fmt.Fprintf(w, "         • %s\n", a)
			}
		}
		fmt.Fprintln(w)
	}

	warnWord := "warnings"
	if warned == 1 {
		warnWord = "warning"
	}
	fmt.Fprintf(w, "Result: %d failed, %d %s, %d passed.\n", failed, warned, warnWord, passed)
	return failed, warned, passed
}

// HandleDoctor executes the diagnostics suite and outputs the formatted health report.
func HandleDoctor(verbose bool) error {
	cwd, _ := os.Getwd()
	globalCfg, _ := config.LoadFile(config.GlobalPath())
	localPath := config.FindLocalPath(cwd)
	var localCfg *config.Config
	if localPath != "" {
		localCfg, _ = config.LoadFile(localPath)
	}
	mergedCfg := config.Merge(globalCfg, localCfg)

	colors := initDoctorColors(shouldUseColor())
	results := RunAllDoctorChecks(mergedCfg, cwd, verbose)
	failed, _, _ := RenderDoctorReport(os.Stdout, results, colors)

	if failed > 0 {
		return fmt.Errorf("doctor detected %d critical check failure(s)", failed)
	}
	return nil
}
