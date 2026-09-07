package learn

import (
	"os"
	"path/filepath"
	"strings"

	"bws/internal/config"
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
	"/run/systemd",
	"/run/udev",
	"/tmp",
	"/var/tmp",
}

var defaultUserPathSubdirs = []string{
	".local/bin",
	"bin",
	".cargo/bin",
	"go/bin",
	".npm-global/bin",
	".yarn/bin",
	".krew/bin",
}

var shellStartupFiles = map[string]bool{
	"~/.bashrc":       true,
	"~/.bash_profile": true,
	"~/.bash_login":   true,
	"~/.bash_logout":  true,
	"~/.profile":      true,
	"~/.inputrc":      true,
	"~/.zshrc":        true,
	"~/.zshenv":       true,
	"~/.zprofile":     true,
	"~/.zlogin":       true,
	"~/.zlogout":      true,
	"~/.cshrc":        true,
	"~/.tcshrc":       true,
	"~/.login":        true,
	"~/.logout":       true,
}

var sensitiveSecretStores = []string{
	"~/.ssh",
	"~/.aws",
	"~/.azure",
	"~/.config/gcloud",
	"~/.password-store",
	"~/.vault-token",
	"~/.gnupg",
	"~/.config/auth",
	"~/.auth",
	"~/.config/gh",
	"~/.local/share/gh",
	"~/.local/state/gh",
	"~/.git-credentials",
	"~/.config/git/credentials",
	"~/.netrc",
	"~/.config/netrc",
	"~/.kube",
	"~/.docker",
	"~/.npmrc",
	"~/.pypirc",
	"~/.cargo/credentials.toml",
	"~/.local/share/keyrings",
	"~/.config/op",
	"~/.terraform.d",
}

func isSensitiveSecretStore(cleanPath, homeDir string) bool {
	norm := NormalizePath(cleanPath, homeDir)
	for _, store := range sensitiveSecretStores {
		if norm == store || strings.HasPrefix(norm, store+"/") {
			return true
		}
	}
	return false
}

func isHistoryOrShellStartup(cleanPath, homeDir string) bool {
	norm := NormalizePath(cleanPath, homeDir)
	if shellStartupFiles[norm] {
		return true
	}
	for _, mask := range config.DefaultHistoryMasks {
		if norm == mask {
			return true
		}
	}
	return false
}

// GetPathDirectories collects all directory paths in the PATH environment variable
// and standard user binary directories (expanded against homeDir).
func GetPathDirectories(homeDir, pathEnv string, extraDirs ...string) []string {
	seen := make(map[string]bool)
	var dirs []string

	addDir := func(d string) {
		d = strings.TrimSpace(d)
		if d == "" {
			return
		}
		if homeDir != "" {
			cleanHome := filepath.Clean(homeDir)
			if d == "~" {
				d = cleanHome
			} else if strings.HasPrefix(d, "~/") {
				d = filepath.Join(cleanHome, d[2:])
			}
		}
		clean := filepath.Clean(d)
		if clean != "" && clean != "." && !seen[clean] {
			seen[clean] = true
			dirs = append(dirs, clean)
		}
	}

	for _, extra := range extraDirs {
		addDir(extra)
	}

	if pathEnv == "" {
		pathEnv = os.Getenv("PATH")
	}
	if pathEnv != "" {
		for _, p := range filepath.SplitList(pathEnv) {
			addDir(p)
		}
	}

	if homeDir != "" {
		cleanHome := filepath.Clean(homeDir)
		for _, sub := range defaultUserPathSubdirs {
			addDir(filepath.Join(cleanHome, sub))
		}
	}

	return dirs
}

// isPathDirOrBinary checks if the given path is a PATH directory or an executable/file inside a PATH directory.
func isPathDirOrBinary(cleanPath string, pathDirs []string) bool {
	for _, pDir := range pathDirs {
		cleanPDir := filepath.Clean(pDir)
		if cleanPDir == "" || cleanPDir == "/" {
			continue
		}
		if cleanPath == cleanPDir || strings.HasPrefix(cleanPath, cleanPDir+"/") {
			return true
		}
	}
	return false
}

// ShouldFilterAccess determines if a traced file path and access mode should be ignored/filtered
// (e.g. system paths, workspace internal paths, PATH directory/executable probes, or read-only /etc accesses).
func ShouldFilterAccess(path string, mode AccessMode, workDir, homeDir string, pathDirs ...string) bool {
	clean := filepath.Clean(strings.TrimSpace(path))
	if clean == "" || clean == "." || clean == "/" {
		return true
	}

	// Filter out the entire home directory if referenced directly
	if homeDir != "" && clean == filepath.Clean(homeDir) {
		return true
	}

	// Filter shell startup files (e.g. .bashrc, .zshrc) and command history files
	if isHistoryOrShellStartup(clean, homeDir) {
		return true
	}

	// Filter sensitive secret stores (e.g. ~/.aws, ~/.gnupg, ~/.config/auth) from auto-mounting
	if isSensitiveSecretStore(clean, homeDir) {
		return true
	}

	// Filter paths within the current workspace (bws mounts workspace CWD automatically)
	if workDir != "" {
		cleanWorkDir := filepath.Clean(workDir)
		if clean == cleanWorkDir || strings.HasPrefix(clean, cleanWorkDir+"/") {
			return true
		}
		// Filter ancestor directories of the workspace.
		// Ancestor directories (e.g. "..", "../..") must not be bound as whole directory mounts,
		// but regular files accessed within an ancestor directory (like ../prefix.tex) are allowed as pinholes.
		if strings.HasPrefix(cleanWorkDir, clean+"/") {
			return true
		}
	}

	// Filter system directories and special runtime paths
	for _, prefix := range defaultFilteredPrefixes {
		if clean == prefix || strings.HasPrefix(clean, prefix+"/") {
			return true
		}
	}

	// Filter all read-only accesses under /etc (libc/glibc initialization, bws manages /etc)
	if clean == "/etc" || strings.HasPrefix(clean, "/etc/") {
		if mode&AccessWrite == 0 {
			return true
		}
	}

	// Filter directory and executable accesses resulting from searching/probing PATH directories
	if len(pathDirs) == 0 {
		pathDirs = GetPathDirectories(homeDir, "")
	}
	if isPathDirOrBinary(clean, pathDirs) {
		if mode&AccessWrite == 0 {
			return true
		}
	}

	return false
}

// ShouldFilterPath determines if a traced file path is a standard system path,
// workspace internal path, PATH directory/executable probe, or ephemeral path that should not be converted into a bind mount.
func ShouldFilterPath(path, workDir, homeDir string, pathDirs ...string) bool {
	return ShouldFilterAccess(path, AccessRead, workDir, homeDir, pathDirs...)
}
