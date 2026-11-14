// Package testutil holds helpers shared by the bws test suites.
package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// ShortSocketDir returns a directory for a test Unix socket, removed when the
// test ends. t.TempDir() nests too deeply under the gate's TMPDIR for the
// 107-byte socket path limit.
func ShortSocketDir(t testing.TB) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "s") // tmp-exempt: test sockets need a short path; t.TempDir is too deep
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// WriteFile writes contents to path, creating its parent directories, and
// fails the test on error.
func WriteFile(t testing.TB, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
