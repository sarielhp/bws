package ssh

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"bws/internal/config"
)

var (
	repoPattern   = regexp.MustCompile(`^([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)$`)
	remotePattern = regexp.MustCompile(`github\.com[:/]+([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+?)(?:\.git)?/*$`)
)

// splitRepo validates "owner/repo" and returns its parts.
func splitRepo(repo string) (owner, name string, ok bool) {
	m := repoPattern.FindStringSubmatch(repo)
	if m == nil {
		return "", "", false
	}
	for _, part := range m[1:] {
		if part == "." || part == ".." {
			return "", "", false
		}
	}
	return m[1], m[2], true
}

// parseGitHubRemote extracts "owner/repo" from a GitHub remote URL.
func parseGitHubRemote(url string) string {
	m := remotePattern.FindStringSubmatch(strings.TrimSpace(url))
	if m == nil {
		return ""
	}
	repo := m[1] + "/" + m[2]
	if _, _, ok := splitRepo(repo); !ok {
		return ""
	}
	return repo
}

// ResolveDeployRepo returns the "owner/repo" whose deploy key the sandbox
// may use, or "" when none applies.
//
// A value pinned in trusted configuration (features.deploy_key_repo) wins.
// Otherwise the workspace's origin URL is consulted, but .git/config is
// writable from inside the sandbox, so the value is bound to the workspace
// on first use in a host-only record. A later origin naming a different
// repository is refused rather than minting or loading a key for it.
func ResolveDeployRepo(pinned, workDir string) (string, error) {
	if pinned != "" {
		if _, _, ok := splitRepo(pinned); !ok {
			return "", fmt.Errorf("invalid features.deploy_key_repo %q (want owner/repo)", pinned)
		}
		return pinned, nil
	}
	top := gitOutput(workDir, "rev-parse", "--show-toplevel")
	if top == "" {
		return "", nil
	}
	repo := parseGitHubRemote(gitOutput(top, "config", "--get", "remote.origin.url"))
	if repo == "" {
		return "", nil
	}
	return checkRepoRecord(top, repo)
}

func checkRepoRecord(top, repo string) (string, error) {
	record := repoRecordPath(top)
	if data, err := os.ReadFile(record); err == nil {
		recorded := strings.TrimSpace(string(data))
		if recorded != repo {
			return "", fmt.Errorf("origin of %s now names %s, but its deploy key is bound to %s; "+
				"if this change is intended, set features.deploy_key_repo to %q and run 'bws config trust'",
				top, repo, recorded, repo)
		}
		return repo, nil
	}
	if err := os.MkdirAll(filepath.Dir(record), 0700); err != nil {
		return "", err
	}
	if err := os.WriteFile(record, []byte(repo+"\n"), 0600); err != nil {
		return "", err
	}
	fmt.Fprintf(os.Stderr, "[bws] Bound deploy-key repository %s to %s\n", repo, top)
	return repo, nil
}

// repoRecordPath lives under the trust directory, which is host-only state
// masked inside every sandbox.
func repoRecordPath(top string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(top)))
	return filepath.Join(config.TrustDir(), fmt.Sprintf("deploy-repo-%x", sum))
}

func gitOutput(dir string, args ...string) string {
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
