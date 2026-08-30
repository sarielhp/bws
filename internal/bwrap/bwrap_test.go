package bwrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bws/internal/config"
	"bws/internal/util"
)

func TestBuildArgsUnshareIPC(t *testing.T) {
	cfg := &config.Config{}
	sandboxDir := t.TempDir()
	currentDir := t.TempDir()

	args := BuildArgs(cfg, sandboxDir, currentDir, true, false)

	foundUnshareIPC := false
	for _, arg := range args {
		if arg == "--unshare-ipc" {
			foundUnshareIPC = true
			break
		}
	}
	if !foundUnshareIPC {
		t.Errorf("expected --unshare-ipc in BuildArgs output, got: %v", args)
	}
}

func TestAddX11ArgsTightened(t *testing.T) {
	t.Setenv("DISPLAY", ":0")
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")

	var args []string
	addX11Args(&args)

	for i := 0; i < len(args); i++ {
		if args[i] == "--bind-try" && i+1 < len(args) && strings.Contains(args[i+1], "/run/user") {
			t.Errorf("found prohibited /run/user bind in addX11Args: %s", args[i+1])
		}
		if args[i] == "--setenv" && i+1 < len(args) && args[i+1] == "XDG_RUNTIME_DIR" {
			t.Errorf("found prohibited XDG_RUNTIME_DIR setenv in addX11Args")
		}
	}

	foundDisplay := false
	foundNoAtSpi := false
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--setenv" && args[i+1] == "DISPLAY" {
			foundDisplay = true
		}
		if args[i] == "--setenv" && args[i+1] == "NO_AT_SPI" {
			foundNoAtSpi = true
		}
	}
	if !foundDisplay {
		t.Error("expected DISPLAY setenv in addX11Args")
	}
	if !foundNoAtSpi {
		t.Error("expected NO_AT_SPI setenv in addX11Args")
	}
}

func TestSSHKnownHostsReadOnly(t *testing.T) {
	home := util.HomeDir()
	sshDir := filepath.Join(home, ".ssh")
	knownHosts := filepath.Join(sshDir, "known_hosts")
	if fi, err := os.Stat(knownHosts); err == nil && !fi.IsDir() {
		cfg := &config.Config{
			Features: &config.FeaturesConfig{
				EnableSSH: boolPtr(true),
			},
		}
		args := BuildArgs(cfg, t.TempDir(), t.TempDir(), true, false)

		for i := 0; i < len(args)-2; i++ {
			if strings.HasSuffix(args[i+1], "known_hosts") {
				if args[i] == "--bind" {
					t.Errorf("known_hosts must not be mounted with --bind (RW), found at index %d", i)
				}
			}
		}
	}
}

func TestAddDBusArgs(t *testing.T) {
	fakeBus := filepath.Join(t.TempDir(), "bus")
	if err := os.WriteFile(fakeBus, []byte("fake"), 0600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+fakeBus)
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1234")

	var args []string
	cfg := &config.Config{
		Features: &config.FeaturesConfig{
			EnableDBus: boolPtr(true),
		},
	}
	addDBusArgs(&args, cfg, true)

	foundBind := false
	foundDBusEnv := false
	foundXDGEnv := false

	for i := 0; i < len(args)-2; i++ {
		if args[i] == "--bind" && args[i+1] == "<filtered-dbus-proxy>" && args[i+2] == "/run/user/1234/bus" {
			foundBind = true
		}
	}
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--setenv" && args[i+1] == "DBUS_SESSION_BUS_ADDRESS" {
			foundDBusEnv = true
		}
		if args[i] == "--setenv" && args[i+1] == "XDG_RUNTIME_DIR" {
			foundXDGEnv = true
		}
	}

	if !foundBind {
		t.Errorf("expected --bind <filtered-dbus-proxy> /run/user/1234/bus in addDBusArgs, got %v", args)
	}
	if !foundDBusEnv {
		t.Error("expected DBUS_SESSION_BUS_ADDRESS setenv in addDBusArgs")
	}
	if !foundXDGEnv {
		t.Error("expected XDG_RUNTIME_DIR setenv in addDBusArgs")
	}
}

func TestDBusDisabled(t *testing.T) {
	fakeBus := filepath.Join(t.TempDir(), "bus")
	_ = os.WriteFile(fakeBus, []byte("fake"), 0600)
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+fakeBus)

	f := false
	cfg := &config.Config{
		Features: &config.FeaturesConfig{
			EnableDBus: &f,
		},
	}
	args := BuildArgs(cfg, t.TempDir(), t.TempDir(), true, false)

	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--setenv" && args[i+1] == "DBUS_SESSION_BUS_ADDRESS" {
			t.Error("DBUS_SESSION_BUS_ADDRESS should not be set when enable_dbus is false")
		}
	}
}

func TestDBusDisabledByDefault(t *testing.T) {
	fakeBus := filepath.Join(t.TempDir(), "bus")
	_ = os.WriteFile(fakeBus, []byte("fake"), 0600)
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+fakeBus)

	cfg := &config.Config{}
	args := BuildArgs(cfg, t.TempDir(), t.TempDir(), true, false)

	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--setenv" && args[i+1] == "DBUS_SESSION_BUS_ADDRESS" {
			t.Error("DBUS_SESSION_BUS_ADDRESS should not be set by default")
		}
	}
}

func TestNoUnconditionalSystemDBus(t *testing.T) {
	cfg := &config.Config{}
	args := BuildArgs(cfg, t.TempDir(), t.TempDir(), true, false)

	for i := 0; i < len(args)-1; i++ {
		if args[i] == "/run/dbus" {
			t.Errorf("unconditional /run/dbus mount must not be present, found at arg index %d", i)
		}
	}
}

func boolPtr(b bool) *bool {
	return &b
}
