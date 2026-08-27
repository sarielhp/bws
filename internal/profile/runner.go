package profile

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"bw/internal/bwrap"
	"bw/internal/config"
	"bw/internal/sandbox"
	"bw/internal/util"
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
	if len(resolved.Tests) == 0 {
		return nil, fmt.Errorf("no tests defined for profile %q", resolved.Name)
	}

	// Apply resolved profile mounts & environment to config for this test run
	testCfg := copyConfig(cfg)
	for _, b := range resolved.BindsRW {
		testCfg.BindsRW = append(testCfg.BindsRW, config.BindEntry{Host: b[0], Sandbox: b[1]})
	}
	for _, b := range resolved.BindsRO {
		testCfg.BindsRO = append(testCfg.BindsRO, config.BindEntry{Host: b[0], Sandbox: b[1]})
	}
	for _, p := range resolved.Path {
		testCfg.Path = append(testCfg.Path, p)
	}
	testCfg.PassEnv = append(testCfg.PassEnv, resolved.PassEnv...)
	testCfg.Mask = append(testCfg.Mask, resolved.Mask...)
	for k, v := range resolved.Env {
		if testCfg.Env == nil {
			testCfg.Env = make(map[string]string)
		}
		testCfg.Env[k] = v
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

	bwrapArgs := bwrap.BuildArgs(testCfg, sandboxDir, currentDir, false, verbose)

	var results []TestResult
	for _, t := range resolved.Tests {
		if len(t.Cmd) == 0 {
			continue
		}
		testName := t.Name
		if testName == "" {
			testName = strings.Join(t.Cmd, " ")
		}

		binName := t.Cmd[0]
		// If binary is not on host and test is marked optional (or standard check), skip it
		if !isBinaryAvailable(binName, resolved.Path) {
			results = append(results, TestResult{
				Name:    testName,
				Command: t.Cmd,
				Status:  "skipped",
				Output:  fmt.Sprintf("binary %q not found on host", binName),
			})
			continue
		}

		start := time.Now()
		cmdArgs := append(append([]string{}, bwrapArgs...), t.Cmd...)
		cmd := exec.Command("bwrap", cmdArgs...)
		out, err := cmd.CombinedOutput()
		dur := time.Since(start)

		outStr := strings.TrimSpace(string(out))
		if err != nil {
			results = append(results, TestResult{
				Name:     testName,
				Command:  t.Cmd,
				Status:   "failed",
				Output:   outStr,
				Duration: dur,
				Error:    err,
			})
		} else {
			results = append(results, TestResult{
				Name:     testName,
				Command:  t.Cmd,
				Status:   "passed",
				Output:   outStr,
				Duration: dur,
			})
		}
	}

	return results, nil
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
