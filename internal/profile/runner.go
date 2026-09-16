package profile

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"bws/internal/bwrap"
	"bws/internal/config"
	"bws/internal/dbus"
	"bws/internal/sandbox"
	"bws/internal/util"
)

// TestResult stores the outcome of an individual test command execution.
type TestResult struct {
	Name     string
	Command  []string
	Status   string // "passed", "skipped", "failed"
	Output   string
	Duration time.Duration
	Error    error
}

// RunProfileTests executes all tests defined in a resolved profile inside the sandbox.
func RunProfileTests(cfg *config.Config, currentDir string, resolved *ResolvedProfile, verbose bool) ([]TestResult, error) {
	if err := util.EnsureBwrap(); err != nil {
		return nil, err
	}
	if len(resolved.Tests) == 0 {
		return nil, fmt.Errorf("no tests defined for profile %q", resolved.Name)
	}
	testCfg := profileTestConfig(cfg, resolved)
	root, _ := config.FindWorkspaceRoot(currentDir)
	config.ExpandWorkspace(testCfg, root)
	if err := config.ValidateWorkspace(currentDir, testCfg.MaxFileCount, false); err != nil {
		return nil, err
	}

	var sandboxDir string
	var cleanup func()
	if testCfg.SandboxPath != "" {
		sandboxDir = util.ExpandHome(testCfg.SandboxPath)
		sandbox.Prepare(testCfg, sandboxDir)
	} else {
		var err error
		sandboxDir, cleanup, err = sandbox.StageHome(testCfg, currentDir)
		if err != nil {
			return nil, fmt.Errorf("failed to stage sandbox home: %w", err)
		}
		defer cleanup()
	}

	var dbusProxy *dbus.Proxy
	if config.FeatureEnabledDefault(testCfg, func(f *config.FeaturesConfig) *bool { return f.EnableDBus }, false) {
		proxy, err := dbus.Start(testCfg, verbose)
		if err == nil && proxy != nil {
			dbusProxy = proxy
			defer dbusProxy.Close()
		}
	}

	bwrapArgs := bwrap.BuildArgs(testCfg, sandboxDir, currentDir, false, verbose)
	if dbusProxy != nil && !dbusProxy.IsRaw() && dbusProxy.SocketPath() != "" {
		bwrapArgs = append(bwrapArgs,
			"--bind", dbusProxy.SocketPath(), dbusProxy.DestPath(),
			"--setenv", "DBUS_SESSION_BUS_ADDRESS", fmt.Sprintf("unix:path=%s", dbusProxy.DestPath()),
			"--setenv", "XDG_RUNTIME_DIR", dbusProxy.DestDir(),
		)
	}

	var results []TestResult
	for _, t := range resolved.Tests {
		if len(t.Cmd) == 0 {
			continue
		}
		results = append(results, runProfileTest(t, resolved.Path, bwrapArgs))
	}

	return results, nil
}

func profileTestConfig(cfg *config.Config, resolved *ResolvedProfile) *config.Config {
	testCfg := copyConfig(cfg)
	testCfg.Features = config.MergeFeatures(testCfg.Features, resolved.Features)
	if resolved.UnshareNet {
		if testCfg.Features == nil {
			testCfg.Features = &config.FeaturesConfig{}
		}
		enabled := true
		testCfg.Features.NoNet = &enabled
	}
	for _, b := range resolved.BindsRW {
		testCfg.BindsRW = append(testCfg.BindsRW, config.BindEntry{Host: b[0], Sandbox: b[1]})
	}
	for _, b := range resolved.BindsRO {
		testCfg.BindsRO = append(testCfg.BindsRO, config.BindEntry{Host: b[0], Sandbox: b[1]})
	}
	testCfg.Path = append(testCfg.Path, resolved.Path...)
	testCfg.PassEnv = append(testCfg.PassEnv, resolved.PassEnv...)
	testCfg.Mask = append(testCfg.Mask, resolved.Mask...)
	if testCfg.Env == nil {
		testCfg.Env = make(map[string]string)
	}
	for k, v := range resolved.Env {
		testCfg.Env[k] = v
	}
	return testCfg
}

func runProfileTest(test TestSpec, paths, bwrapArgs []string) TestResult {
	name := test.Name
	if name == "" {
		name = strings.Join(test.Cmd, " ")
	}
	result := TestResult{Name: name, Command: test.Cmd, Status: "passed"}
	if !isBinaryAvailable(test.Cmd[0], paths) {
		result.Status = "skipped"
		result.Output = fmt.Sprintf("binary %q not found on host", test.Cmd[0])
		return result
	}
	start := time.Now()
	cmd := exec.Command("bwrap", append(append([]string{}, bwrapArgs...), test.Cmd...)...)
	out, err := cmd.CombinedOutput()
	result.Output = strings.TrimSpace(string(out))
	result.Duration = time.Since(start)
	result.Error = err
	if err != nil {
		result.Status = "failed"
	}
	return result
}

func isBinaryAvailable(bin string, extraPaths []string) bool {
	if strings.Contains(bin, "/") {
		expanded := util.ExpandHome(bin)
		_, err := os.Stat(expanded)
		return err == nil
	}

	if util.CommandExists(bin) {
		return true
	}

	for _, p := range extraPaths {
		expanded := util.ExpandHome(p)
		target := strings.Replace(expanded, "@@HOME@@", util.HomeDir(), -1)
		full := target + "/" + bin
		if fi, err := os.Stat(full); err == nil && !fi.IsDir() {
			return true
		}
	}

	return false
}

func copyConfig(c *config.Config) *config.Config {
	if c == nil {
		return &config.Config{}
	}
	cp := *c
	cp.BindsRW = append([]config.BindEntry{}, c.BindsRW...)
	cp.BindsRO = append([]config.BindEntry{}, c.BindsRO...)
	cp.Path = append([]string{}, c.Path...)
	if c.Env != nil {
		cp.Env = make(map[string]string)
		for k, v := range c.Env {
			cp.Env[k] = v
		}
	}
	return &cp
}
