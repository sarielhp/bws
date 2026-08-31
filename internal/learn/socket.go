package learn

import (
	"os"
	"strings"
)

// DetectSocketFeatures inspects socket addresses and connects to detect required sandbox features.
func DetectSocketFeatures(sockAddr string, features *DetectedFeatures) {
	if sockAddr == "" || features == nil {
		return
	}

	// Network socket detection (TCP/UDP)
	if strings.HasPrefix(sockAddr, "AF_INET:") || strings.HasPrefix(sockAddr, "AF_INET6:") {
		features.Net = true
		return
	}

	if !strings.HasPrefix(sockAddr, "AF_UNIX:") {
		return
	}

	unixPath := strings.TrimPrefix(sockAddr, "AF_UNIX:")

	// D-Bus session or system bus detection
	if strings.Contains(unixPath, "/bus") ||
		strings.Contains(unixPath, "dbus") ||
		strings.Contains(unixPath, "system_bus_socket") ||
		strings.Contains(unixPath, "session_bus_socket") {
		features.DBus = true
	}

	// SSH Agent detection
	sshAuthSock := os.Getenv("SSH_AUTH_SOCK")
	if (sshAuthSock != "" && unixPath == sshAuthSock) ||
		strings.Contains(unixPath, "/ssh-") ||
		strings.Contains(unixPath, "agent.") ||
		strings.Contains(unixPath, "/keyring/ssh") ||
		strings.Contains(unixPath, "openssh_agent") {
		features.SSH = true
	}

	// X11 display socket detection
	if strings.Contains(unixPath, ".X11-unix") ||
		strings.Contains(unixPath, ".X11-pipe") ||
		strings.Contains(unixPath, "/X11/") {
		features.X11 = true
	}

	// WSL interop detection
	if strings.Contains(unixPath, "/run/WSL") ||
		strings.Contains(unixPath, "WSL_INTEROP") {
		features.WSL = true
	}
}

// DetectPathFeatures checks file paths accessed during tracing for special subsystems like WSL.
func DetectPathFeatures(path string, features *DetectedFeatures) {
	if path == "" || features == nil {
		return
	}

	if strings.HasPrefix(path, "/run/WSL") ||
		strings.HasPrefix(path, "/mnt/c/") ||
		strings.HasPrefix(path, "/mnt/d/") ||
		strings.Contains(path, "WSL_INTEROP") {
		features.WSL = true
	}
}
