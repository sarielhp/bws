package bwrap

import (
	_ "embed"
	"os"
	"path/filepath"
)

//go:embed git_export.sh
var gitExportScript string

// GitExportArgs confines all reads of agent-controlled Git metadata to an offline
// sandbox. The bundle leaves through stdout; no host output directory is exposed.
func GitExportArgs(cloneDir, branch string) []string {
	args := []string{
		"--unshare-all", "--clearenv", "--new-session", "--die-with-parent",
		"--ro-bind", "/usr", "/usr",
		"--ro-bind", "/bin", "/bin",
		"--ro-bind", "/lib", "/lib",
		"--ro-bind-try", "/lib64", "/lib64",
		"--tmpfs", "/tmp", "--tmpfs", "/home", "--tmpfs", "/etc",
		"--proc", "/proc", "--dev", "/dev",
		"--bind", cloneDir, "/workspace", "--chdir", "/workspace",
		"--tmpfs", "/workspace/.bws",
		"--setenv", "HOME", "/home", "--setenv", "PATH", "/usr/bin:/bin",
		"--setenv", "GIT_CONFIG_NOSYSTEM", "1",
		"--setenv", "GIT_CONFIG_GLOBAL", "/dev/null",
	}
	if _, err := os.Lstat(filepath.Join(cloneDir, ".bws.jsonc")); err == nil {
		args = append(args, "--ro-bind", "/dev/null", "/workspace/.bws.jsonc")
	}
	return append(args, "/bin/sh", "-c", gitExportScript, "bws-git-export", branch)
}
