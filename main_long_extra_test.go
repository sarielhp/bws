//go:build long

package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestEmacsInSandbox(t *testing.T) {
	if exec.Command("emacs", "--version").Run() != nil {
		t.Skip("emacs not found on host, skipping")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	output := sandboxRun(t, ctx, "-f", "--", "emacs", "--batch", "--eval", "(message \"Hello from Emacs in sandbox!\")")
	if !bytes.Contains(output, []byte("Hello from Emacs in sandbox!")) {
		t.Errorf("unexpected output: %s", string(output))
	}
}

func TestPythonInSandbox(t *testing.T) {
	if exec.Command("python3", "--version").Run() != nil {
		t.Skip("python3 not found on host, skipping")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	output := sandboxRun(t, ctx, "-f", "--", "python3", "-c", "print('Hello from Python in sandbox!')")
	if !bytes.Contains(output, []byte("Hello from Python in sandbox!")) {
		t.Errorf("unexpected output: %s", string(output))
	}
}

func TestMakeInSandbox(t *testing.T) {
	if exec.Command("make", "--version").Run() != nil {
		t.Skip("make not found on host, skipping")
	}

	makefile := `all:
	@echo "Hello from Make in sandbox!"
`
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Makefile"), []byte(makefile), 0644)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bwPath, "-f", "--", "make")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil && ctx.Err() != nil {
		t.Fatal("timed out")
	}
	if err != nil {
		t.Fatalf("make failed: %v\n%s", err, string(output))
	}
	if !bytes.Contains(output, []byte("Hello from Make in sandbox!")) {
		t.Errorf("unexpected output: %s", string(output))
	}
}

func TestQuartoInSandbox(t *testing.T) {
	if exec.Command("quarto", "--version").Run() != nil {
		t.Skip("quarto not found on host, skipping")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	output := sandboxRun(t, ctx, "-f", "--", "quarto", "--version")
	if len(bytes.TrimSpace(output)) == 0 {
		t.Error("quarto --version produced no output")
	}
}

func TestUVInSandbox(t *testing.T) {
	if exec.Command("uv", "--version").Run() != nil {
		t.Skip("uv not found on host, skipping")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bwPath, "-f", "--", "uv", "--version")
	cmd.Dir = t.TempDir()
	uvOut, err := cmd.CombinedOutput()
	if err != nil && ctx.Err() != nil {
		t.Fatal("timed out")
	}
	if err != nil {
		t.Fatalf("uv --version failed: %v\n%s", err, string(uvOut))
	}

	cmd2 := exec.CommandContext(ctx, bwPath, "-f", "--", "uvx", "--version")
	cmd2.Dir = t.TempDir()
	uvxOut, err2 := cmd2.CombinedOutput()
	if err2 != nil && ctx.Err() != nil {
		t.Fatal("timed out")
	}
	if err2 != nil {
		t.Fatalf("uvx --version failed: %v\n%s", err2, string(uvxOut))
	}

	if len(bytes.TrimSpace(uvOut)) == 0 {
		t.Error("uv --version produced no output")
	}
	if len(bytes.TrimSpace(uvxOut)) == 0 {
		t.Error("uvx --version produced no output")
	}
}
