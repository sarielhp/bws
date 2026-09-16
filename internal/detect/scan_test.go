package detect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanBoundaries(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"go.mod", "web/package.json", "node_modules/tool/Cargo.toml", ".git/hooks/payload", "a/b/c/d/too-deep"} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("not executed"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "pyproject.toml"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "external")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "go.mod"), filepath.Join(root, "Cargo.toml")); err != nil {
		t.Fatal(err)
	}
	e, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, v := range e.Entries {
		seen[v.Path] = true
	}
	for _, name := range []string{"go.mod", "web/package.json"} {
		if !seen[name] {
			t.Errorf("missing %s", name)
		}
	}
	for _, name := range []string{"external", "Cargo.toml", "node_modules/tool/Cargo.toml", ".git/hooks/payload", "a/b/c/d/too-deep"} {
		if seen[name] {
			t.Errorf("scanned excluded entry %s", name)
		}
	}
}

func TestScanBudget(t *testing.T) {
	root := t.TempDir()
	if err := scanDirectoryMustExceed(root); err == nil {
		t.Fatal("expected entry budget failure")
	}
}

func scanDirectoryMustExceed(dir string) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	f, err := root.Create("marker")
	if err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	count := MaxEntries
	return scanDirectory(root, ".", 0, &Evidence{}, &count)
}
