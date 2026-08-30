package dbus

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// HostSessionBusAddress returns the session D-Bus address on the host, or empty string if not found.
func HostSessionBusAddress() string {
	addr := os.Getenv("DBUS_SESSION_BUS_ADDRESS")
	if addr != "" {
		return addr
	}
	candidate := fmt.Sprintf("/run/user/%d/bus", os.Getuid())
	if fi, err := os.Stat(candidate); err == nil && (fi.Mode()&os.ModeSocket != 0 || !fi.IsDir()) {
		return fmt.Sprintf("unix:path=%s", candidate)
	}
	return ""
}

// HostSessionBusSocketPath extracts the file path of the UNIX domain socket from the host address.
func HostSessionBusSocketPath() string {
	addr := HostSessionBusAddress()
	if addr == "" {
		return ""
	}
	if strings.HasPrefix(addr, "unix:path=") {
		sock := strings.TrimPrefix(addr, "unix:path=")
		if idx := strings.Index(sock, ","); idx != -1 {
			sock = sock[:idx]
		}
		return sock
	}
	return ""
}

// SandboxDestinationPaths returns the standard destination socket path and runtime directory inside the container.
func SandboxDestinationPaths() (string, string) {
	uid := os.Getuid()
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = fmt.Sprintf("/run/user/%d", uid)
	}
	busPath := filepath.Join(runtimeDir, "bus")
	return busPath, runtimeDir
}
