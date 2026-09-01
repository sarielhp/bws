package learn

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"bws/internal/util"
)

// RunTrace executes the target command under strace and analyzes captured syscalls.
func RunTrace(opts TraceOptions) (*TraceResult, error) {
	if len(opts.Command) == 0 {
		return nil, fmt.Errorf("no command specified for learning")
	}

	if !util.CommandExists("strace") {
		return nil, fmt.Errorf("strace is required for bws learn but was not found in PATH")
	}

	if opts.HomeDir == "" {
		opts.HomeDir = util.HomeDir()
	}
	if opts.WorkDir == "" {
		if pwd, err := os.Getwd(); err == nil {
			opts.WorkDir = pwd
		}
	}

	// 1. Binary PATH discovery
	var discoveredPath string
	if binDir, err := ResolveBinaryDir(opts.Command[0], opts.WorkDir, opts.HomeDir); err == nil && binDir != "" {
		discoveredPath = binDir
	}

	tmpFile, err := os.CreateTemp("", "bws-learn-*.log")
	if err != nil {
		return nil, fmt.Errorf("creating temporary trace log file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	traceSyscalls := "open,openat,creat,unlink,unlinkat,rename,renameat,renameat2,mkdir,mkdirat,connect,bind,stat,lstat,newfstatat,statx,access,faccessat,faccessat2,openat2,truncate,ftruncate"
	straceArgs := []string{
		"-f",
		"-e", "trace=" + traceSyscalls,
		"-s", "1024",
		"-o", tmpPath,
		"--",
	}
	straceArgs = append(straceArgs, opts.Command...)

	cmd := exec.Command("strace", straceArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	runErr := cmd.Run()
	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("executing trace: %w", runErr)
		}
	}

	logFile, err := os.Open(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("opening trace log file: %w", err)
	}
	defer logFile.Close()

	res, err := AnalyzeTrace(logFile, opts)
	if err != nil {
		return nil, err
	}
	res.ExitCode = exitCode
	res.Command = opts.Command
	res.DiscoveredPath = discoveredPath

	return res, nil
}

// AnalyzeTrace reads strace log lines from an io.Reader and generates a TraceResult.
func AnalyzeTrace(r io.Reader, opts TraceOptions) (*TraceResult, error) {
	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading trace log: %w", err)
	}

	return AnalyzeTraceLines(lines, opts), nil
}

// AnalyzeTraceLines parses a slice of strace log lines into a TraceResult.
func AnalyzeTraceLines(lines []string, opts TraceOptions) *TraceResult {
	if opts.HomeDir == "" {
		opts.HomeDir = util.HomeDir()
	}
	if opts.WorkDir == "" {
		if pwd, err := os.Getwd(); err == nil {
			opts.WorkDir = pwd
		}
	}

	pathDirs := GetPathDirectories(opts.HomeDir, opts.PathEnv, opts.PathDirs...)

	features := DetectedFeatures{}
	accesses := make(map[string]AccessMode)

	for _, line := range lines {
		parsed := ParseTraceLine(line)
		if parsed == nil {
			continue
		}

		if parsed.SockAddr != "" {
			DetectSocketFeatures(parsed.SockAddr, &features)
		}

		for _, path := range parsed.Paths {
			absPath := path
			if !filepath.IsAbs(path) && opts.WorkDir != "" {
				absPath = filepath.Join(opts.WorkDir, path)
			}

			DetectPathFeatures(absPath, &features)

			if ShouldFilterAccess(absPath, parsed.Mode, opts.WorkDir, opts.HomeDir, pathDirs...) {
				continue
			}

			accesses[absPath] |= parsed.Mode
		}
	}

	rawRW, bindsRO := CollapseAndClassify(accesses, opts.HomeDir)
	safeRW, alerts := FilterSensitiveWrites(rawRW, opts.HomeDir)

	return &TraceResult{
		Command:        opts.Command,
		Features:       features,
		BindsRW:        safeRW,
		BindsRO:        bindsRO,
		SecurityAlerts: alerts,
		AllAccesses:    accesses,
	}
}
