package dbus

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"bws/internal/config"
	"bws/internal/util"
)

// DefaultTalkPolicies defines the default allowed D-Bus interfaces when proxy filtering is enabled.
var DefaultTalkPolicies = []string{"org.freedesktop.secrets"}

// Proxy represents an active filtered D-Bus proxy instance.
type Proxy struct {
	cmd         *exec.Cmd
	tempDir     string
	hostSocket  string
	proxySocket string
	destPath    string
	destDir     string
	isRaw       bool
	closed      bool
	mu          sync.Mutex
}

func validateHostBus(verbose bool) (string, string, bool) {
	hostAddr := HostSessionBusAddress()
	if hostAddr == "" {
		if verbose {
			fmt.Fprintf(os.Stderr, "[verbose] No session D-Bus socket found on host\n")
		}
		return "", "", false
	}
	hostSock := HostSessionBusSocketPath()
	if hostSock == "" {
		return "", "", false
	}
	if fi, err := os.Stat(hostSock); err != nil || (fi.Mode()&os.ModeSocket == 0 && fi.IsDir()) {
		if verbose {
			fmt.Fprintf(os.Stderr, "[verbose] Host D-Bus socket %s does not exist or is invalid: %v\n", hostSock, err)
		}
		return "", "", false
	}
	return hostAddr, hostSock, true
}

func handleMissingProxy(cfg *config.Config, hostSock, destPath, destDir string) (*Proxy, error) {
	allowRaw := config.FeatureEnabledDefault(cfg, func(f *config.FeaturesConfig) *bool {
		return f.AllowRawDBus
	}, false)

	if allowRaw {
		fmt.Fprintf(os.Stderr, "[bws] SECURITY WARNING: xdg-dbus-proxy is not installed; mounting raw unproxied host D-Bus socket.\n")
		fmt.Fprintf(os.Stderr, "[bws] This allows sandboxed processes to invoke host systemd and access all stored credentials!\n")
		return &Proxy{
			hostSocket:  hostSock,
			proxySocket: hostSock,
			destPath:    destPath,
			destDir:     destDir,
			isRaw:       true,
		}, nil
	}

	fmt.Fprintf(os.Stderr, "[bws] Warning: enable_dbus is enabled, but xdg-dbus-proxy is not installed.\n")
	fmt.Fprintf(os.Stderr, "[bws] D-Bus access is disabled to prevent sandbox escape (install xdg-dbus-proxy or set allow_raw_dbus: true).\n")
	return nil, nil
}

func buildProxyArgs(cfg *config.Config, hostAddr, proxySock string) []string {
	talkPolicies := DefaultTalkPolicies
	if cfg != nil && cfg.Features != nil && len(cfg.Features.DBusTalk) > 0 {
		talkPolicies = cfg.Features.DBusTalk
	}
	proxyArgs := []string{hostAddr, proxySock, "--filter"}
	for _, talk := range talkPolicies {
		proxyArgs = append(proxyArgs, "--talk="+talk)
	}
	return proxyArgs
}

func waitForProxySocket(cmd *exec.Cmd, proxySock, tmpDir string) error {
	for i := 0; i < 100; i++ {
		time.Sleep(10 * time.Millisecond)
		if fi, err := os.Stat(proxySock); err == nil && (fi.Mode()&os.ModeSocket != 0 || !fi.IsDir()) {
			return nil
		}
		if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
			break
		}
	}
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
	_ = os.RemoveAll(tmpDir)
	return fmt.Errorf("xdg-dbus-proxy failed to initialize socket at %s", proxySock)
}

// Start launches xdg-dbus-proxy to filter session D-Bus traffic.
func Start(cfg *config.Config, verbose bool) (*Proxy, error) {
	hostAddr, hostSock, ok := validateHostBus(verbose)
	if !ok {
		return nil, nil
	}

	destPath, destDir := SandboxDestinationPaths()
	if !util.CommandExists("xdg-dbus-proxy") {
		return handleMissingProxy(cfg, hostSock, destPath, destDir)
	}

	tmpDir, err := util.UserTempDir("dbus_proxy")
	if err != nil {
		return nil, fmt.Errorf("creating dbus proxy directory: %w", err)
	}

	proxySock := filepath.Join(tmpDir, "bus")
	proxyArgs := buildProxyArgs(cfg, hostAddr, proxySock)

	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] Launching D-Bus proxy: xdg-dbus-proxy %s\n", strings.Join(proxyArgs, " "))
	}

	cmd := exec.Command("xdg-dbus-proxy", proxyArgs...)
	if err := cmd.Start(); err != nil {
		_ = os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("starting xdg-dbus-proxy: %w", err)
	}

	if err := waitForProxySocket(cmd, proxySock, tmpDir); err != nil {
		return nil, err
	}

	return &Proxy{
		cmd:         cmd,
		tempDir:     tmpDir,
		hostSocket:  hostSock,
		proxySocket: proxySock,
		destPath:    destPath,
		destDir:     destDir,
		isRaw:       false,
	}, nil
}

// SocketPath returns the host filesystem path of the proxy socket.
func (p *Proxy) SocketPath() string {
	if p == nil {
		return ""
	}
	return p.proxySocket
}

// DestPath returns the container path where the socket should be mounted.
func (p *Proxy) DestPath() string {
	if p == nil {
		return ""
	}
	return p.destPath
}

// DestDir returns the directory containing the socket inside the container.
func (p *Proxy) DestDir() string {
	if p == nil {
		return ""
	}
	return p.destDir
}

// IsRaw returns true if this proxy instance is using a raw unproxied host socket.
func (p *Proxy) IsRaw() bool {
	if p == nil {
		return false
	}
	return p.isRaw
}

// Close gracefully terminates the xdg-dbus-proxy process and cleans up its directory.
func (p *Proxy) Close() error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}
	p.closed = true

	var lastErr error
	if p.cmd != nil && p.cmd.Process != nil {
		if err := p.cmd.Process.Kill(); err != nil {
			// explicitly ignored
		}
		if err := p.cmd.Wait(); err != nil {
			// explicitly ignored
		}
	}

	if p.tempDir != "" {
		if err := os.RemoveAll(p.tempDir); err != nil {
			lastErr = err
		}
	}

	return lastErr
}
