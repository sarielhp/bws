package trace

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
		return nil, fmt.Errorf("no command specified for tracing")
	}

	if !util.CommandExists("strace") {
		return nil, fmt.Errorf("strace is required for bws trace but was not found in PATH")
	}

	if opts.HomeDir == "" {
		opts.HomeDir = util.HomeDir()
	}
	if opts.WorkDir == "" {
		opts.WorkDir, _ = os.Getwd()
	}

	tmpFile, err := os.CreateTemp("", "bws-trace-*.log")
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
		opts.WorkDir, _ = os.Getwd()
	}

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

			if ShouldFilterPath(absPath, opts.WorkDir, opts.HomeDir) {
				continue
			}

			accesses[absPath] |= parsed.Mode
		}
	}

	bindsRW, bindsRO := CollapseAndClassify(accesses, opts.HomeDir)

	return &TraceResult{
		Command:     opts.Command,
		Features:    features,
		BindsRW:     bindsRW,
		BindsRO:     bindsRO,
		AllAccesses: accesses,
	}
}
