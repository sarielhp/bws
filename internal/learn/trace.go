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

	normalizeTraceOptions(&opts)

	var initialPath string
	if binDir, err := ResolveBinaryDir(opts.Command[0], opts.WorkDir, opts.HomeDir); err == nil && binDir != "" {
		initialPath = binDir
	}

	tmpPath, err := createTempLog()
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpPath)

	exitCode, err := executeStrace(opts.Command, tmpPath)
	if err != nil {
		return nil, err
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

	if initialPath != "" && !containsString(res.DiscoveredPaths, initialPath) {
		res.DiscoveredPaths = append([]string{initialPath}, res.DiscoveredPaths...)
	}
	if res.DiscoveredPath == "" && len(res.DiscoveredPaths) > 0 {
		res.DiscoveredPath = res.DiscoveredPaths[0]
	}

	return res, nil
}

func normalizeTraceOptions(opts *TraceOptions) {
	if opts.HomeDir == "" {
		opts.HomeDir = util.HomeDir()
	}
	if opts.WorkDir == "" {
		if pwd, err := os.Getwd(); err == nil {
			opts.WorkDir = pwd
		}
	}
	if realWorkDir, err := filepath.EvalSymlinks(opts.WorkDir); err == nil {
		opts.WorkDir = realWorkDir
	}
}

func createTempLog() (string, error) {
	tmpFile, err := os.CreateTemp("", "bws-learn-*.log")
	if err != nil {
		return "", fmt.Errorf("creating temporary trace log file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	return tmpPath, nil
}

func executeStrace(command []string, tmpPath string) (int, error) {
	traceSyscalls := "open,openat,creat,unlink,unlinkat,rename,renameat,renameat2,mkdir,mkdirat,connect,bind,stat,lstat,newfstatat,statx,access,faccessat,faccessat2,openat2,truncate,ftruncate,execve,execveat"
	straceArgs := []string{
		"-f",
		"-e", "trace=" + traceSyscalls,
		"-s", "1024",
		"-o", tmpPath,
		"--",
	}
	straceArgs = append(straceArgs, command...)

	cmd := exec.Command("strace", straceArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	runErr := cmd.Run()
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return 0, fmt.Errorf("executing trace: %w", runErr)
	}
	return 0, nil
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
	normalizeTraceOptions(&opts)
	pathDirs := GetPathDirectories(opts.HomeDir, opts.PathEnv, opts.PathDirs...)

	features := DetectedFeatures{}
	accesses := make(map[string]AccessMode)

	var discoveredPaths []string
	discoveredSeen := make(map[string]bool)
	addDiscovered := func(dir string) {
		if dir == "" || discoveredSeen[dir] {
			return
		}
		discoveredSeen[dir] = true
		discoveredPaths = append(discoveredPaths, dir)
	}

	parser := NewTraceParser()
	for _, line := range lines {
		parsed := parser.ParseLine(line)
		if parsed == nil {
			continue
		}

		if parsed.SockAddr != "" {
			DetectSocketFeatures(parsed.SockAddr, &features)
		}

		if !parsed.Success {
			continue
		}

		if parsed.Name == "execve" || parsed.Name == "execveat" {
			for _, binPath := range parsed.Paths {
				if binDir, err := ResolveBinaryDir(binPath, opts.WorkDir, opts.HomeDir); err == nil && binDir != "" {
					addDiscovered(binDir)
				}
			}
		}

		processParsedPaths(parsed, opts, pathDirs, &features, accesses)
	}

	if len(discoveredPaths) == 0 && len(opts.Command) > 0 {
		if binDir, err := ResolveBinaryDir(opts.Command[0], opts.WorkDir, opts.HomeDir); err == nil && binDir != "" {
			addDiscovered(binDir)
		}
	}

	rawRW, bindsRO := CollapseAndClassify(accesses, opts.HomeDir)
	safeRW, alerts := FilterSensitiveWrites(rawRW, opts.HomeDir)

	var primaryPath string
	if len(discoveredPaths) > 0 {
		primaryPath = discoveredPaths[0]
	}

	return &TraceResult{
		Command:         opts.Command,
		Features:        features,
		BindsRW:         safeRW,
		BindsRO:         bindsRO,
		DiscoveredPath:  primaryPath,
		DiscoveredPaths: discoveredPaths,
		SecurityAlerts:  alerts,
		AllAccesses:     accesses,
	}
}

func processParsedPaths(parsed *ParsedSyscall, opts TraceOptions, pathDirs []string, features *DetectedFeatures, accesses map[string]AccessMode) {
	for _, path := range parsed.Paths {
		absPath := path
		if !filepath.IsAbs(path) && opts.WorkDir != "" {
			absPath = filepath.Join(opts.WorkDir, path)
		}
		if realAbs, err := filepath.EvalSymlinks(absPath); err == nil {
			absPath = realAbs
		}

		DetectPathFeatures(absPath, features)

		if ShouldFilterAccess(absPath, parsed.Mode, opts.WorkDir, opts.HomeDir, pathDirs...) {
			continue
		}

		accesses[absPath] |= parsed.Mode
	}
}

func containsString(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}
