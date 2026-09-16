package sandbox

import (
	"os"
	"path/filepath"
	"testing"

	"bws/internal/config"
)

func TestSkeletonRejectsHostSymlinks(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	secret := filepath.Join(t.TempDir(), "host-secret")
	if err := os.WriteFile(secret, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"file", "skeleton", "parent"} {
		t.Run(kind, func(t *testing.T) {
			project := t.TempDir()
			skeleton := LocalSkeletonDir(project)
			if err := os.MkdirAll(filepath.Dir(skeleton), 0755); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "file":
				if err := os.Mkdir(skeleton, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(secret, filepath.Join(skeleton, "leak")); err != nil {
					t.Fatal(err)
				}
			case "skeleton":
				if err := os.Symlink(filepath.Dir(secret), skeleton); err != nil {
					t.Fatal(err)
				}
			case "parent":
				if err := os.Remove(filepath.Dir(skeleton)); err != nil {
					t.Fatal(err)
				}
				outside := t.TempDir()
				if err := os.Mkdir(filepath.Join(outside, "skeleton"), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, filepath.Dir(skeleton)); err != nil {
					t.Fatal(err)
				}
			}
			_, cleanup, err := StageHome(&config.Config{}, project)
			if cleanup != nil {
				defer cleanup()
			}
			if err == nil {
				t.Fatal("host symlink accepted")
			}
		})
	}
}

func TestSkeletonAllowsInternalFileLink(t *testing.T) {
	src, dest := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "source"), []byte("safe"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("source", filepath.Join(src, "link")); err != nil {
		t.Fatal(err)
	}
	if err := copyDirectoryContents(src, dest); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dest, "link"))
	if err != nil || string(data) != "safe" {
		t.Fatalf("copy = %q, %v", data, err)
	}
}
