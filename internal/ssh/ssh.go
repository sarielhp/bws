package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"bws/internal/util"
)

// Agent is a private ssh-agent serving a single sandbox session.
type Agent struct {
	cmd  *exec.Cmd
	dir  string
	sock string
}

var (
	sessionMu     sync.Mutex
	sessionAgents []*Agent
)

const agentStartTimeout = 3 * time.Second

// maxSocketPath is the longest Unix socket path Linux accepts (sun_path minus its NUL).
const maxSocketPath = 107

// StartSessionAgent starts a private ssh-agent for one sandbox session and
// loads exactly the given keys. Each session gets its own socket, so
// concurrent sandboxes never see each other's keys, and default identities
// from ~/.ssh are never loaded implicitly. It returns "" when no key could be
// loaded. Call StopSessionAgents when the session ends.
func StartSessionAgent(keys []string) string {
	if len(keys) == 0 || !util.CommandExists("ssh-agent") || !util.CommandExists("ssh-add") {
		return ""
	}
	dir, err := util.UserTempDir("ssh")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[bws] Warning: SSH agent disabled: %v\n", err)
		return ""
	}
	// The socket is bound at the same path inside the sandbox, and a Unix
	// socket path is limited to 107 bytes, so its name is kept short.
	a := &Agent{dir: dir, sock: filepath.Join(dir, "s")}
	if len(a.sock) > maxSocketPath {
		_ = os.RemoveAll(dir)
		fmt.Fprintf(os.Stderr, "[bws] Warning: SSH agent disabled: socket path %s is longer than %d bytes; use a shorter TMPDIR\n", a.sock, maxSocketPath)
		return ""
	}
	a.cmd = exec.Command("ssh-agent", "-D", "-a", a.sock)
	// Own process group so terminal signals do not reach it; Pdeathsig
	// stops it if bws dies without running its cleanup.
	a.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGTERM}
	if err := a.cmd.Start(); err != nil {
		_ = os.RemoveAll(dir)
		fmt.Fprintf(os.Stderr, "[bws] Warning: cannot start ssh-agent: %v\n", err)
		return ""
	}
	if !waitForSocket(a.sock, agentStartTimeout) {
		a.stop()
		fmt.Fprintf(os.Stderr, "[bws] Warning: ssh-agent did not create its socket\n")
		return ""
	}
	if addKeys(a.sock, keys) == 0 {
		a.stop()
		return ""
	}
	sessionMu.Lock()
	sessionAgents = append(sessionAgents, a)
	sessionMu.Unlock()
	return a.sock
}

// StopSessionAgents terminates every agent started by StartSessionAgent.
func StopSessionAgents() {
	sessionMu.Lock()
	agents := sessionAgents
	sessionAgents = nil
	sessionMu.Unlock()
	for _, a := range agents {
		a.stop()
	}
}

func (a *Agent) stop() {
	if a.cmd != nil && a.cmd.Process != nil {
		_ = a.cmd.Process.Signal(syscall.SIGTERM)
		_ = a.cmd.Wait()
	}
	_ = os.RemoveAll(a.dir)
}

func waitForSocket(path string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fi, err := os.Stat(path); err == nil && fi.Mode()&os.ModeSocket != 0 {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
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

// addKeys loads the given keys into the agent and returns how many loaded.
func addKeys(agentSock string, keys []string) int {
	loaded := 0
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
			continue
		}
		loaded++
	}
	return loaded
}

// GetAutoDeployKey returns the dedicated deploy key for repo ("owner/repo"),
// generating it and registering it with GitHub on first use. The caller must
// obtain repo from ResolveDeployRepo, never directly from the workspace.
func GetAutoDeployKey(repo string) string {
	owner, name, ok := splitRepo(repo)
	if !ok || !util.CommandExists("gh") {
		return ""
	}

	keyDir := filepath.Join(util.HomeDir(), ".sandbox", "deploy_keys")
	if err := ensurePrivateDir(keyDir); err != nil {
		fmt.Fprintf(os.Stderr, "[bws] Warning: cannot create deploy key directory: %v\n", err)
		return ""
	}
	keyPath := filepath.Join(keyDir, owner+"_"+name)

	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "[bws] Creating dedicated GitHub Deploy Key for %s/%s...\n", owner, name)
		cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-f", keyPath, "-N", "", "-C", fmt.Sprintf("bw-auto-deploy-%s-%s", owner, name))
		if cmd.Run() == nil {
			fmt.Fprintf(os.Stderr, "[bws] Registering Deploy Key with GitHub repository via gh CLI...\n")
			title := fmt.Sprintf("bw-auto-deploy-%s", name)
			if err := exec.Command("gh", "repo", "deploy-key", "add", keyPath+".pub", "-R", owner+"/"+name, "-w", "-t", title).Run(); err != nil {
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
