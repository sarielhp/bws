package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalPolicyTrust(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, name := range []string{".bws/config.jsonc", ".bws/config.json", ".bws.jsonc", "child/.bws/config.jsonc"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), name)
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(`{"binds_rw":["/"]}`), 0644); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadLocalFile(path); err == nil {
				t.Fatal("unapproved policy accepted")
			}
			if err := TrustFile(path); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadLocalFile(path); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(`{"binds_rw":["/home"]}`), 0644); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadLocalFile(path); err == nil {
				t.Fatal("modified policy accepted")
			}
			if err := SetConfigKV(path, "max_file_count", "5"); err == nil {
				t.Fatal("editing laundered untrusted policy")
			}
		})
	}
}

func TestWorkspaceBoundariesAndCounts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, parent := range []string{home, filepath.Join(home, "bin")} {
		if err := os.MkdirAll(filepath.Join(parent, ".bws"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(parent, ".bws", "config.jsonc"), []byte("{}"), 0644); err != nil {
			t.Fatal(err)
		}
		child := filepath.Join(parent, "child")
		if err := os.MkdirAll(child, 0755); err != nil {
			t.Fatal(err)
		}
		if root, _ := FindWorkspaceRoot(child); root != child {
			t.Fatalf("protected ancestor selected: %s", root)
		}
		if err := ValidateWorkspace(parent, 1000, true); err == nil {
			t.Fatal("force bypassed protected directory")
		}
	}
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, ".bws"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, ".bws", "config.jsonc"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(workspace, "small")
	if err := os.Mkdir(child, 0755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := os.WriteFile(filepath.Join(workspace, fmt.Sprint(i)), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := ValidateWorkspace(child, 2, false); err == nil {
		t.Fatal("ancestor file count bypassed")
	}
	if err := ValidateWorkspace(child, 2, true); err != nil {
		t.Fatal(err)
	}
}

func TestTrustCannotBeBorrowedThroughDirectoryLink(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	approved, other := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(approved, ".bws"), 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(approved, ".bws", "config.jsonc")
	if err := WriteTrustedFile(path, []byte("{}")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(approved, ".bws"), filepath.Join(other, ".bws")); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadLocalFile(filepath.Join(other, ".bws", "config.jsonc")); err == nil {
		t.Fatal("borrowed another workspace's approval")
	}
}
