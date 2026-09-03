package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bws/internal/learn"
)

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	outChan := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		outChan <- buf.String()
	}()

	fnErr := fn()
	_ = w.Close()
	os.Stdout = oldStdout
	out := <-outChan
	return out, fnErr
}

func sampleTraceResult() *learn.TraceResult {
	return &learn.TraceResult{
		Command:  []string{"dummy", "app"},
		ExitCode: 0,
		Features: learn.DetectedFeatures{
			Net:  true,
			SSH:  true,
			DBus: false,
			X11:  false,
			WSL:  false,
		},
		DiscoveredPath: "/opt/tools/bin",
		BindsRW:        []string{"/home/user/project/data"},
		BindsRO:        []string{"/usr/lib", "/etc/ssl"},
	}
}

func TestHandleDeltaDispatch_EmptyDelta_NonVerbose(t *testing.T) {
	res := sampleTraceResult()
	delta := &learn.Delta{}

	out, err := captureStdout(t, func() error {
		return handleDeltaDispatch(res, delta, "/tmp/dummy.jsonc", "", false, false, false, false)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out, "✓ Sandbox configuration already covers all required access. No changes needed.") {
		t.Errorf("expected success message in output, got:\n%s", out)
	}
	if strings.Contains(out, "Detected Features:") {
		t.Errorf("Detected Features should be omitted when delta is empty and verbose=false, got:\n%s", out)
	}
	if strings.Contains(out, "Discovered Bind Mounts:") {
		t.Errorf("Discovered Bind Mounts should be omitted when delta is empty and verbose=false, got:\n%s", out)
	}
	if strings.Contains(out, "Binary PATH Discovery:") {
		t.Errorf("Binary PATH Discovery should be omitted when delta is empty and verbose=false, got:\n%s", out)
	}
}

func TestHandleDeltaDispatch_EmptyDelta_Verbose(t *testing.T) {
	res := sampleTraceResult()
	delta := &learn.Delta{}

	out, err := captureStdout(t, func() error {
		return handleDeltaDispatch(res, delta, "/tmp/dummy.jsonc", "", false, false, false, true)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out, "✓ Sandbox configuration already covers all required access. No changes needed.") {
		t.Errorf("expected completion message in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Detected Features:") {
		t.Errorf("expected Detected Features in verbose output, got:\n%s", out)
	}
	if !strings.Contains(out, "Discovered Bind Mounts:") {
		t.Errorf("expected Discovered Bind Mounts in verbose output, got:\n%s", out)
	}
	if !strings.Contains(out, "Binary PATH Discovery:") {
		t.Errorf("expected Binary PATH Discovery in verbose output, got:\n%s", out)
	}
}

func TestHandleDeltaDispatch_SecurityAlerts_NonVerbose(t *testing.T) {
	res := sampleTraceResult()
	delta := &learn.Delta{
		SecurityAlerts: []string{"[SECURITY ALERT] Disallowed read access to /etc/shadow"},
	}

	out, err := captureStdout(t, func() error {
		return handleDeltaDispatch(res, delta, "/tmp/dummy.jsonc", "", false, false, false, false)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out, "[SECURITY ALERT] Disallowed read access to /etc/shadow") {
		t.Errorf("expected security alert to be printed even when verbose is false, got:\n%s", out)
	}
	if !strings.Contains(out, "✓ Sandbox configuration already covers all required access. No changes needed.") {
		t.Errorf("expected success message in output, got:\n%s", out)
	}
	if strings.Contains(out, "Detected Features:") {
		t.Errorf("Detected Features should NOT be printed when verbose=false, got:\n%s", out)
	}
}

func TestHandleDeltaDispatch_NonEmptyDelta_DryRun(t *testing.T) {
	res := sampleTraceResult()
	delta := &learn.Delta{
		BindsRW: []string{"/home/user/project/new_rw"},
	}

	// Non-verbose dry-run
	outNonVerbose, err := captureStdout(t, func() error {
		return handleDeltaDispatch(res, delta, "/tmp/dummy.jsonc", "", false, false, true, false)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(outNonVerbose, "Detected Features:") {
		t.Errorf("non-verbose dry run should not print full trace features, got:\n%s", outNonVerbose)
	}
	if !strings.Contains(outNonVerbose, "Discovered sandbox configuration additions for") {
		t.Errorf("expected additions header in dry run, got:\n%s", outNonVerbose)
	}
	if !strings.Contains(outNonVerbose, "/home/user/project/new_rw") {
		t.Errorf("expected delta bind in dry run output, got:\n%s", outNonVerbose)
	}

	// Verbose dry-run
	outVerbose, err := captureStdout(t, func() error {
		return handleDeltaDispatch(res, delta, "/tmp/dummy.jsonc", "", false, false, true, true)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(outVerbose, "Detected Features:") {
		t.Errorf("verbose dry run should print Detected Features, got:\n%s", outVerbose)
	}
	if !strings.Contains(outVerbose, "Discovered sandbox configuration additions for") {
		t.Errorf("expected additions header in verbose dry run, got:\n%s", outVerbose)
	}
}

func TestHandleDeltaDispatch_NonEmptyDelta_LiveMerge(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, ".bws", "config.jsonc")

	res := sampleTraceResult()
	delta := &learn.Delta{
		BindsRW: []string{"/home/user/project/new_rw"},
	}

	out, err := captureStdout(t, func() error {
		return handleDeltaDispatch(res, delta, targetPath, "", false, false, false, false)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(out, "Detected Features:") {
		t.Errorf("non-verbose live merge should not dump detected features, got:\n%s", out)
	}
	if !strings.Contains(out, "✓ Updated local configuration") {
		t.Errorf("expected updated message in output, got:\n%s", out)
	}
	if !strings.Contains(out, "• Added 1 read-write bind mounts") {
		t.Errorf("expected added rw count in output, got:\n%s", out)
	}
	if !strings.Contains(out, "/home/user/project/new_rw") {
		t.Errorf("expected added rw path in output, got:\n%s", out)
	}
}
