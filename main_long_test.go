//go:build long
// +build long

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

func skipIfNoBinary(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}
}

func sandboxRun(t *testing.T, ctx context.Context, args ...string) []byte {
	t.Helper()
	cmd := exec.CommandContext(ctx, bwPath, args...)
	cmd.Dir = t.TempDir()
	output, err := cmd.CombinedOutput()
	if err != nil && ctx.Err() != nil {
		t.Fatal("timed out")
	}
	if err != nil {
		t.Fatalf("sandbox command failed: %v\n%s", err, string(output))
	}
	return output
}

func TestLaTeXInSandbox(t *testing.T) {
	if err := exec.Command("lualatex", "--version").Run(); err != nil {
		t.Skip("lualatex does not work on host, skipping")
	}

	minimalLatex := `\documentclass{article}
\begin{document}
Hello, sandbox!
\end{document}
`

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.tex"), []byte(minimalLatex), 0644)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bwPath, "-f", "--", "lualatex", "-interaction=nonstopmode", "test.tex")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil && ctx.Err() != nil {
		t.Fatal("timed out")
	}
	if err != nil {
		t.Fatalf("sandbox lualatex failed: %v\n%s", err, string(out))
	}

	// Also run on host for comparison
	hostDir := t.TempDir()
	os.WriteFile(filepath.Join(hostDir, "test.tex"), []byte(minimalLatex), 0644)
	cmd2 := exec.CommandContext(ctx, "lualatex", "-interaction=nonstopmode", "test.tex")
	cmd2.Dir = hostDir
	if out, err := cmd2.CombinedOutput(); err != nil {
		t.Fatalf("host lualatex failed: %v\n%s", err, string(out))
	}

	// Validate both PDFs
	insidePDF, _ := os.ReadFile(filepath.Join(dir, "test.pdf"))
	hostPDF, _ := os.ReadFile(filepath.Join(hostDir, "test.pdf"))

	if len(insidePDF) == 0 || !bytes.HasPrefix(insidePDF, []byte("%PDF")) {
		t.Error("sandbox PDF is invalid or empty")
	}
	if len(hostPDF) == 0 || !bytes.HasPrefix(hostPDF, []byte("%PDF")) {
		t.Error("host PDF is invalid or empty")
	}
	if len(insidePDF) < 500 {
		t.Errorf("sandbox PDF suspiciously small: %d bytes", len(insidePDF))
	}
	if len(hostPDF) < 500 {
		t.Errorf("host PDF suspiciously small: %d bytes", len(hostPDF))
	}
}

func TestOpenCodeInSandbox(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	opencodePath := filepath.Join(homeDir, ".opencode", "bin", "opencode")
	if _, err := os.Stat(opencodePath); os.IsNotExist(err) {
		t.Skipf("opencode not found at %s, skipping", opencodePath)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	sandboxRun(t, ctx, "-f", "--", opencodePath, "debug", "info")
}

func TestCppInSandbox(t *testing.T) {
	cxx := ""
	for _, bin := range []string{"g++", "clang++"} {
		if exec.Command(bin, "--version").Run() == nil {
			cxx = bin
			break
		}
	}
	if cxx == "" {
		t.Skip("neither g++ nor clang++ found on host, skipping")
	}

	cppSrc := `#include <iostream>
int main() {
	std::cout << "Hello from C++ in sandbox!" << std::endl;
	return 0;
}
`
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.cpp"), []byte(cppSrc), 0644)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bwPath, "-f", "--", cxx, "-std=c++17", "-o", "test", "test.cpp")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil && ctx.Err() != nil {
		t.Fatal("timed out")
	}
	if err != nil {
		t.Fatalf("compilation failed: %v\n%s", err, string(out))
	}

	cmd2 := exec.CommandContext(ctx, bwPath, "-f", "--", "./test")
	cmd2.Dir = dir
	out2, err := cmd2.CombinedOutput()
	if err != nil && ctx.Err() != nil {
		t.Fatal("timed out")
	}
	if err != nil {
		t.Fatalf("running compiled C++ binary failed: %v\n%s", err, string(out2))
	}
	if !bytes.Contains(out2, []byte("Hello from C++ in sandbox!")) {
		t.Errorf("unexpected output: %s", string(out2))
	}
}

func TestRustInSandbox(t *testing.T) {
	if exec.Command("rustc", "--version").Run() != nil {
		t.Skip("rustc not found on host, skipping")
	}

	rustSrc := `fn main() {
	println!("Hello from Rust in sandbox!");
}
`
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.rs"), []byte(rustSrc), 0644)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bwPath, "-f", "--", "rustc", "-o", "test", "test.rs")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil && ctx.Err() != nil {
		t.Fatal("timed out")
	}
	if err != nil {
		t.Fatalf("compilation failed: %v\n%s", err, string(out))
	}

	cmd2 := exec.CommandContext(ctx, bwPath, "-f", "--", "./test")
	cmd2.Dir = dir
	out2, err := cmd2.CombinedOutput()
	if err != nil && ctx.Err() != nil {
		t.Fatal("timed out")
	}
	if err != nil {
		t.Fatalf("running compiled Rust binary failed: %v\n%s", err, string(out2))
	}
	if !bytes.Contains(out2, []byte("Hello from Rust in sandbox!")) {
		t.Errorf("unexpected output: %s", string(out2))
	}
}

func TestGoInSandbox(t *testing.T) {
	if exec.Command("go", "version").Run() != nil {
		t.Skip("go not found on host, skipping")
	}

	goSrc := `package main
import "fmt"
func main() {
	fmt.Println("Hello from Go in sandbox!")
}
`
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(goSrc), 0644)
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module hello\ngo 1.22\n"), 0644)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Build in dir
	cmd := exec.CommandContext(ctx, bwPath, "-f", "--", "go", "build", "-o", "hello", ".")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil && ctx.Err() != nil {
		t.Fatal("timed out")
	}
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, string(output))
	}

	// Run the binary from dir
	cmd2 := exec.CommandContext(ctx, bwPath, "-f", "--", "./hello")
	cmd2.Dir = dir
	out2, err := cmd2.CombinedOutput()
	if err != nil && ctx.Err() != nil {
		t.Fatal("timed out")
	}
	if err != nil {
		t.Fatalf("running compiled Go binary failed: %v\n%s", err, string(out2))
	}
	if !bytes.Contains(out2, []byte("Hello from Go in sandbox!")) {
		t.Errorf("unexpected output: %s", string(out2))
	}
}

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
