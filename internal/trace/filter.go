package trace

import (
	"path/filepath"
	"strings"
)

var defaultFilteredPrefixes = []string{
	"/proc",
	"/sys",
	"/dev",
	"/bin",
	"/sbin",
	"/lib",
	"/lib64",
	"/lib32",
	"/libx32",
	"/usr",
	"/etc/ld.so.cache",
	"/etc/ld.so.preload",
	"/etc/ld.so.conf",
	"/etc/resolv.conf",
	"/etc/hosts",
	"/etc/nsswitch.conf",
	"/etc/gai.conf",
	"/etc/ssl",
	"/etc/pki",
	"/etc/ca-certificates",
	"/etc/localtime",
	"/etc/timezone",
	"/etc/passwd",
	"/etc/group",
	"/run/systemd",
	"/run/udev",
	"/tmp",
	"/var/tmp",
}

// ShouldFilterPath determines if a traced file path is a standard system path,
// workspace internal path, or ephemeral path that should not be converted into a bind mount.
func ShouldFilterPath(path, workDir, homeDir string) bool {
	clean := filepath.Clean(strings.TrimSpace(path))
	if clean == "" || clean == "." || clean == "/" {
		return true
	}

	// Filter out the entire home directory if referenced directly
	if clean == filepath.Clean(homeDir) {
		return true
	}

	// Filter paths within the current workspace (bws mounts workspace CWD automatically)
	if workDir != "" {
		cleanWorkDir := filepath.Clean(workDir)
		if clean == cleanWorkDir || strings.HasPrefix(clean, cleanWorkDir+"/") {
			return true
		}
	}

	// Filter system directories and special runtime paths
	for _, prefix := range defaultFilteredPrefixes {
		if clean == prefix || strings.HasPrefix(clean, prefix+"/") {
			return true
		}
	}

	return false
}
