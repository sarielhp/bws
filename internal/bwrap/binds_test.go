package bwrap

import (
	"os"
	"path/filepath"
	"testing"

	"bws/internal/config"
	"bws/internal/util"
)

func findArgIndex(args []string, match ...string) int {
	if len(match) == 0 {
		return -1
	}
	for i := 0; i <= len(args)-len(match); i++ {
		matched := true
		for j := 0; j < len(match); j++ {
			if args[i+j] != match[j] {
				matched = false
				break
			}
		}
		if matched {
			return i
		}
	}
	return -1
}

func TestNestedMountOrderingWorkspacePrecedesChildMounts(t *testing.T) {
	currentDir := t.TempDir()
	childDir := filepath.Join(currentDir, "nested", "data")
	if err := os.MkdirAll(childDir, 0755); err != nil {
		t.Fatal(err)
	}

	hostSource := t.TempDir()
	cfg := &config.Config{
		BindsRO: []config.BindEntry{
			{Host: hostSource, Sandbox: childDir},
		},
	}

	sandboxDir := t.TempDir()
	args := BuildArgs(cfg, sandboxDir, currentDir, true, false)

	idxWorkspace := findArgIndex(args, "--bind", currentDir, currentDir)
	if idxWorkspace == -1 {
		t.Fatalf("expected --bind %s %s in args", currentDir, currentDir)
	}

	idxChild := findArgIndex(args, "--ro-bind", hostSource, childDir)
	if idxChild == -1 {
		t.Fatalf("expected child sub-mount --ro-bind %s %s in args: %v", hostSource, childDir, args)
	}

	if idxWorkspace >= idxChild {
		t.Errorf("workspace mount (%d) must precede child sub-mount (%d) to prevent shadowing", idxWorkspace, idxChild)
	}
}

func TestNestedMountOrderingROInsideRW(t *testing.T) {
	tmpDir := t.TempDir()
	parentHost := filepath.Join(tmpDir, "host_parent")
	childHost := filepath.Join(tmpDir, "host_child")
	_ = os.MkdirAll(parentHost, 0755)
	_ = os.MkdirAll(childHost, 0755)

	parentDest := "/opt/app"
	childDest := "/opt/app/readonly_config"

	cfg := &config.Config{
		BindsRW: []config.BindEntry{
			{Host: parentHost, Sandbox: parentDest},
		},
		BindsRO: []config.BindEntry{
			{Host: childHost, Sandbox: childDest},
		},
	}

	sandboxDir := t.TempDir()
	currentDir := t.TempDir()
	args := BuildArgs(cfg, sandboxDir, currentDir, true, false)

	idxParent := findArgIndex(args, "--bind", parentHost, parentDest)
	idxChild := findArgIndex(args, "--ro-bind", childHost, childDest)

	if idxParent == -1 || idxChild == -1 {
		t.Fatalf("failed to find parent (%d) or child (%d) mount in args: %v", idxParent, idxChild, args)
	}

	if idxParent >= idxChild {
		t.Errorf("parent RW mount (%d) must precede child RO mount (%d)", idxParent, idxChild)
	}
}

func TestNestedMountOrderingRWInsideRO(t *testing.T) {
	tmpDir := t.TempDir()
	parentHost := filepath.Join(tmpDir, "host_parent")
	childHost := filepath.Join(tmpDir, "host_child")
	_ = os.MkdirAll(parentHost, 0755)
	_ = os.MkdirAll(childHost, 0755)

	parentDest := "/opt/app"
	childDest := "/opt/app/writable_cache"

	cfg := &config.Config{
		BindsRO: []config.BindEntry{
			{Host: parentHost, Sandbox: parentDest},
		},
		BindsRW: []config.BindEntry{
			{Host: childHost, Sandbox: childDest},
		},
	}

	sandboxDir := t.TempDir()
	currentDir := t.TempDir()
	args := BuildArgs(cfg, sandboxDir, currentDir, true, false)

	idxParent := findArgIndex(args, "--ro-bind", parentHost, parentDest)
	idxChild := findArgIndex(args, "--bind", childHost, childDest)

	if idxParent == -1 || idxChild == -1 {
		t.Fatalf("failed to find parent (%d) or child (%d) mount in args: %v", idxParent, idxChild, args)
	}

	if idxParent >= idxChild {
		t.Errorf("parent RO mount (%d) must precede child RW mount (%d)", idxParent, idxChild)
	}
}

func TestBuildBindsEqualDepthROPrecedesRW(t *testing.T) {
	tmpDir := t.TempDir()
	hostRO := filepath.Join(tmpDir, "host_ro")
	hostRW := filepath.Join(tmpDir, "host_rw")
	_ = os.MkdirAll(hostRO, 0755)
	_ = os.MkdirAll(hostRW, 0755)

	dest := "/mnt/data"

	cfg := &config.Config{
		BindsRW: []config.BindEntry{
			{Host: hostRW, Sandbox: dest},
		},
		BindsRO: []config.BindEntry{
			{Host: hostRO, Sandbox: dest},
		},
	}

	args := buildBinds(cfg, "", util.HomeDir(), tmpDir, false)

	idxRO := findArgIndex(args, "--ro-bind", hostRO, dest)
	idxRW := findArgIndex(args, "--bind", hostRW, dest)

	if idxRO == -1 || idxRW == -1 {
		t.Fatalf("failed to find RO (%d) or RW (%d) in buildBinds: %v", idxRO, idxRW, args)
	}

	if idxRO >= idxRW {
		t.Errorf("when depth is equal, RO mount (%d) must precede RW mount (%d)", idxRO, idxRW)
	}
}

func TestBaseParentMountsPrecedeUserBindsAndMasksLast(t *testing.T) {
	tmpDir := t.TempDir()
	userHost := filepath.Join(tmpDir, "user_host")
	_ = os.MkdirAll(userHost, 0755)
	userDest := "/tmp/custom_sub"

	cfg := &config.Config{
		BindsRO: []config.BindEntry{
			{Host: userHost, Sandbox: userDest},
		},
	}

	sandboxDir := t.TempDir()
	currentDir := t.TempDir()
	args := BuildArgs(cfg, sandboxDir, currentDir, true, false)

	idxTmp := findArgIndex(args, "/tmp")
	idxUser := findArgIndex(args, userDest)

	if idxTmp == -1 || idxUser == -1 {
		t.Fatalf("missing /tmp (%d) or userDest (%d) in args", idxTmp, idxUser)
	}

	if idxTmp >= idxUser {
		t.Errorf("base /tmp mount (%d) must precede user mount beneath it (%d)", idxTmp, idxUser)
	}

	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--ro-bind-try" && i+1 < len(args) && args[i+1] == "/dev/null" {
			if i < idxUser {
				t.Errorf("mask arg at index %d must not precede user bind at %d", i, idxUser)
			}
		}
	}
}
