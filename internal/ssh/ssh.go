package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"bws/internal/util"
)

// EnsureAgent starts or reuses the sandbox SSH agent and loads keys into it.
func EnsureAgent(keys []string) string {
	if !util.CommandExists("ssh-agent") || !util.CommandExists("ssh-add") {
		return ""
	}

	agentDir := filepath.Join(util.HomeDir(), ".sandbox")
	if err := ensurePrivateDir(agentDir); err != nil {
		fmt.Fprintf(os.Stderr, "[bws] Warning: SSH agent disabled: %v\n", err)
		return ""
	}
	agentSock := filepath.Join(agentDir, "ssh_agent.sock")

	if fi, err := os.Stat(agentSock); err == nil && fi.Mode()&os.ModeSocket != 0 {
		cmd := exec.Command("ssh-add", "-l")
		cmd.Env = append(os.Environ(), "SSH_AUTH_SOCK="+agentSock)
		err := cmd.Run()
		if err == nil || cmd.ProcessState.ExitCode() == 1 {
			os.Setenv("SSH_AUTH_SOCK", agentSock)
			if len(keys) > 0 {
				if err := exec.Command("ssh-add", "-D").Run(); err != nil {
					fmt.Fprintf(os.Stderr, "[bws] Warning: failed to clear SSH keys: %v\n", err)
				}
			}
			addKeys(agentSock, keys)
			return agentSock
		}
		os.Remove(agentSock)
	}

	cmd := exec.Command("ssh-agent", "-a", agentSock)
	if cmd.Run() == nil {
		os.Setenv("SSH_AUTH_SOCK", agentSock)
		addKeys(agentSock, keys)
		return agentSock
	}

	return ""
}

// ensurePrivateDir creates dir if needed and forces it to mode 0700, since
// MkdirAll leaves the mode of an existing directory unchanged.
func ensurePrivateDir(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	return os.Chmod(dir, 0700)
}

// keyPath resolves an ssh_keys entry: absolute paths are used as-is and
// relative ones are taken relative to the home directory.
func keyPath(k string) string {
	if filepath.IsAbs(k) {
		return filepath.Clean(k)
	}
	return filepath.Join(util.HomeDir(), k)
}

// addKeys loads the given keys into the agent, or the default keys when none
// are configured.
func addKeys(agentSock string, keys []string) {
	if len(keys) == 0 {
		cmd := exec.Command("ssh-add")
		cmd.Env = append(os.Environ(), "SSH_AUTH_SOCK="+agentSock)
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "[bws] Warning: failed to add default SSH keys: %v\n", err)
		}
		return
	}
	for _, k := range keys {
		expK := keyPath(k)
		if fi, err := os.Stat(expK); err != nil || fi.IsDir() {
			fmt.Fprintf(os.Stderr, "[bws] Warning: SSH key not found: %s\n", expK)
			continue
		}
		cmd := exec.Command("ssh-add", expK)
		cmd.Env = append(os.Environ(), "SSH_AUTH_SOCK="+agentSock)
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "[bws] Warning: failed to add SSH key %s: %v\n", expK, err)
		}
	}
}

func GetAutoDeployKey() string {
	if !util.CommandExists("git") || !util.CommandExists("gh") {
		return ""
	}

	remote, err := exec.Command("git", "config", "--get", "remote.origin.url").Output()
	if err != nil {
		return ""
	}
	remoteURL := strings.TrimSpace(string(remote))
	if remoteURL == "" {
		return ""
	}

	var owner, repo string
	patterns := []string{
		"github.com[:/]([^/]+)/([^/.]+)",
	}
	for _, pat := range patterns {
		re := regexp.MustCompile(pat)
		matches := re.FindStringSubmatch(remoteURL)
		if len(matches) >= 3 {
			owner = matches[1]
			repo = strings.TrimSuffix(matches[2], ".git")
			break
		}
	}
	if owner == "" || repo == "" {
		return ""
	}

	keyDir := filepath.Join(util.HomeDir(), ".sandbox", "deploy_keys")
	if err := ensurePrivateDir(keyDir); err != nil {
		fmt.Fprintf(os.Stderr, "[bws] Warning: cannot create deploy key directory: %v\n", err)
		return ""
	}
	keyPath := filepath.Join(keyDir, owner+"_"+repo)

	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "[bws] Creating dedicated GitHub Deploy Key for %s/%s...\n", owner, repo)
		cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-f", keyPath, "-N", "", "-C", fmt.Sprintf("bw-auto-deploy-%s-%s", owner, repo))
		if cmd.Run() == nil {
			fmt.Fprintf(os.Stderr, "[bws] Registering Deploy Key with GitHub repository via gh CLI...\n")
			title := fmt.Sprintf("bw-auto-deploy-%s", repo)
			if err := exec.Command("gh", "repo", "deploy-key", "add", keyPath+".pub", "-R", fmt.Sprintf("%s/%s", owner, repo), "-w", "-t", title).Run(); err != nil {
				fmt.Fprintf(os.Stderr, "[bws] Warning: failed to register Deploy Key: %v\n", err)
			}
		} else {
			fmt.Fprintf(os.Stderr, "[bws] Warning: failed to generate SSH key\n")
		}
	}

	if _, err := os.Stat(keyPath); err == nil {
		return keyPath
	}
	return ""
}
