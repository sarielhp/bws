package main

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIDBusFlagsAndPlan(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}
	ensureGlobalConfig(t)

	fakeDir := t.TempDir()
	fakeSock := filepath.Join(fakeDir, "bus")
	l, err := net.Listen("unix", fakeSock)
	if err != nil {
		t.Fatalf("failed to create unix socket: %v", err)
	}
	defer l.Close()

	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+fakeSock)

	// Test 1: By default, D-Bus is disabled
	cmdDefault := exec.Command(bwPath, "plan")
	cmdDefault.Dir = t.TempDir()
	outDefault, err := cmdDefault.CombinedOutput()
	if err != nil {
		t.Fatalf("bws plan failed: %v\n%s", err, string(outDefault))
	}
	if strings.Contains(string(outDefault), "DBUS_SESSION_BUS_ADDRESS") {
		t.Errorf("DBUS_SESSION_BUS_ADDRESS should not be present in default plan, got:\n%s", string(outDefault))
	}

	// Test 2: With --dbus flag, filtered proxy is planned
	cmdDBus := exec.Command(bwPath, "plan", "--dbus")
	cmdDBus.Dir = t.TempDir()
	outDBus, err := cmdDBus.CombinedOutput()
	if err != nil {
		t.Fatalf("bws plan --dbus failed: %v\n%s", err, string(outDBus))
	}
	if !strings.Contains(string(outDBus), "DBUS_SESSION_BUS_ADDRESS") {
		t.Errorf("expected DBUS_SESSION_BUS_ADDRESS in bws plan --dbus output, got:\n%s", string(outDBus))
	}
	if !strings.Contains(string(outDBus), "<filtered-dbus-proxy>") {
		t.Errorf("expected <filtered-dbus-proxy> in bws plan --dbus output, got:\n%s", string(outDBus))
	}

	// Test 3: With --no-dbus flag, D-Bus is explicitly suppressed
	cmdNoDBus := exec.Command(bwPath, "plan", "--dbus", "--no-dbus")
	cmdNoDBus.Dir = t.TempDir()
	outNoDBus, err := cmdNoDBus.CombinedOutput()
	if err != nil {
		t.Fatalf("bws plan --dbus --no-dbus failed: %v\n%s", err, string(outNoDBus))
	}
	if strings.Contains(string(outNoDBus), "DBUS_SESSION_BUS_ADDRESS") {
		t.Errorf("DBUS_SESSION_BUS_ADDRESS should not be present with --no-dbus, got:\n%s", string(outNoDBus))
	}
}

func TestCLIAgyProfileDBus(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}
	ensureGlobalConfig(t)

	fakeDir := t.TempDir()
	fakeSock := filepath.Join(fakeDir, "bus")
	l, err := net.Listen("unix", fakeSock)
	if err != nil {
		t.Fatalf("failed to create unix socket: %v", err)
	}
	defer l.Close()

	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+fakeSock)

	workDir := t.TempDir()
	cmdInit := exec.Command(bwPath, "init", "-f", "-p", "agy", workDir)
	if out, err := cmdInit.CombinedOutput(); err != nil {
		t.Fatalf("init with agy profile failed: %v\n%s", err, string(out))
	}

	cmdPlan := exec.Command(bwPath, "plan")
	cmdPlan.Dir = workDir
	outPlan, err := cmdPlan.CombinedOutput()
	if err != nil {
		t.Fatalf("bws plan in agy workspace failed: %v\n%s", err, string(outPlan))
	}
	if !strings.Contains(string(outPlan), "DBUS_SESSION_BUS_ADDRESS") {
		t.Errorf("expected DBUS_SESSION_BUS_ADDRESS in agy workspace plan, got:\n%s", string(outPlan))
	}
}
