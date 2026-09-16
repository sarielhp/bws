package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicPolicyWrite(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), ".bws", "profiles", "test.json")
	first := []byte("{\"name\":\"test\"}\n")
	if err := AtomicPolicyWrite(path, first, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadTrustedFile(path); err != nil {
		t.Fatal(err)
	}
	if err := AtomicPolicyWrite(path, first, nil); err == nil {
		t.Fatal("overwrote create-only destination")
	}
	if err := AtomicPolicyWrite(path, first, []byte("stale")); err == nil {
		t.Fatal("accepted stale preview")
	}
	second := []byte("{\"name\":\"test\",\"description\":\"updated\"}\n")
	if err := AtomicPolicyWrite(path, second, first); err != nil {
		t.Fatal(err)
	}
	got, err := ReadTrustedFile(path)
	if err != nil || string(got) != string(second) {
		t.Fatalf("wrong approved bytes: %s %v", got, err)
	}
}

func TestAtomicPolicyRefusesSymlinks(t *testing.T) {
	for _, parent := range []bool{false, true} {
		t.Run(map[bool]string{true: "parent", false: "file"}[parent], func(t *testing.T) {
			root, outside := t.TempDir(), t.TempDir()
			target := filepath.Join(root, "profile.json")
			if parent {
				if err := os.Symlink(outside, filepath.Join(root, "profiles")); err != nil {
					t.Fatal(err)
				}
				target = filepath.Join(root, "profiles", "profile.json")
			} else {
				if err := os.Symlink(filepath.Join(outside, "untouched"), target); err != nil {
					t.Fatal(err)
				}
			}
			if err := AtomicPolicyWrite(target, []byte("{}"), nil); err == nil {
				t.Fatal("accepted symlink")
			}
			entries, err := os.ReadDir(outside)
			if err != nil || len(entries) != 0 {
				t.Fatal("modified symlink target")
			}
		})
	}
}
