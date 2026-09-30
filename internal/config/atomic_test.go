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

func TestAtomicPolicyAllowsSymlinkedAncestor(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()
	realBase := filepath.Join(tmp, "real", "nested", "workspace")
	if err := os.MkdirAll(realBase, 0755); err != nil {
		t.Fatal(err)
	}
	symlinkBase := filepath.Join(tmp, "shortcut")
	if err := os.Symlink(filepath.Join(tmp, "real"), symlinkBase); err != nil {
		t.Fatal(err)
	}

	wsViaSymlink := filepath.Join(symlinkBase, "nested", "workspace")
	policyPath := filepath.Join(wsViaSymlink, ".bws", "config.jsonc")
	data := []byte("{\"binds_rw\":[]}\n")
	if err := AtomicPolicyWrite(policyPath, data, nil); err != nil {
		t.Fatalf("failed to write policy through symlinked ancestor: %v", err)
	}
	got, err := ReadTrustedFile(policyPath)
	if err != nil || string(got) != string(data) {
		t.Fatalf("wrong trusted policy bytes: got %s, err %v", got, err)
	}
}

func TestAtomicPolicyRefusesSymlinkedBwsDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(ws, ".bws")); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(ws, ".bws", "config.jsonc")
	if err := AtomicPolicyWrite(target, []byte("{}"), nil); err == nil {
		t.Fatal("expected error when .bws is a symlink, got nil")
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatal("modified symlinked .bws target directory")
	}
}
