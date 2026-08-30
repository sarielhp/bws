package dbus

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"bws/internal/config"
	"bws/internal/util"
)

func TestHostSessionBusDetection(t *testing.T) {
	fakeDir := t.TempDir()
	fakeSock := filepath.Join(fakeDir, "test.sock")
	if err := os.WriteFile(fakeSock, []byte(""), 0600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+fakeSock+",guid=12345")

	addr := HostSessionBusAddress()
	if addr != "unix:path="+fakeSock+",guid=12345" {
		t.Errorf("expected %q, got %q", "unix:path="+fakeSock+",guid=12345", addr)
	}

	sockPath := HostSessionBusSocketPath()
	if sockPath != fakeSock {
		t.Errorf("expected sockPath %q, got %q", fakeSock, sockPath)
	}
}

func TestSandboxDestinationPaths(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/9999")
	busPath, runtimeDir := SandboxDestinationPaths()

	if runtimeDir != "/run/user/9999" {
		t.Errorf("expected runtimeDir /run/user/9999, got %s", runtimeDir)
	}
	if busPath != "/run/user/9999/bus" {
		t.Errorf("expected busPath /run/user/9999/bus, got %s", busPath)
	}
}

func TestStartNoHostBus(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "")
	nonExistent := filepath.Join(t.TempDir(), "nonexistent.sock")
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+nonExistent)

	cfg := &config.Config{
		Features: &config.FeaturesConfig{
			EnableDBus: boolPtr(true),
		},
	}

	p, err := Start(cfg, false)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if p != nil {
		t.Errorf("expected nil proxy when host bus doesn't exist, got %+v", p)
	}
}

func TestStartAndCloseProxyWithRealSocket(t *testing.T) {
	if !util.CommandExists("xdg-dbus-proxy") {
		t.Skip("xdg-dbus-proxy not installed, skipping live proxy test")
	}

	fakeDir := t.TempDir()
	fakeSock := filepath.Join(fakeDir, "bus")
	l, err := net.Listen("unix", fakeSock)
	if err != nil {
		t.Fatalf("failed to create unix socket: %v", err)
	}
	defer l.Close()

	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+fakeSock)

	cfg := &config.Config{
		Features: &config.FeaturesConfig{
			EnableDBus: boolPtr(true),
			DBusTalk:   []string{"org.freedesktop.secrets", "org.freedesktop.portal.Secret"},
		},
	}

	p, err := Start(cfg, false)
	if err != nil {
		t.Fatalf("failed to start proxy: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil proxy")
	}
	defer p.Close()

	if p.IsRaw() {
		t.Error("expected filtered proxy, got raw")
	}
	if p.SocketPath() == "" {
		t.Error("expected non-empty SocketPath")
	}
	if _, err := os.Stat(p.SocketPath()); err != nil {
		t.Errorf("proxy socket does not exist on disk: %v", err)
	}

	// Test Close cleans up socket
	sockDir := p.tempDir
	if err := p.Close(); err != nil {
		t.Errorf("error during close: %v", err)
	}
	if _, err := os.Stat(sockDir); !os.IsNotExist(err) {
		t.Errorf("expected proxy temp dir to be removed after Close()")
	}
}

func boolPtr(b bool) *bool {
	return &b
}
