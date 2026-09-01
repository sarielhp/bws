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

// Start launches xdg-dbus-proxy to filter session D-Bus traffic.
func Start(cfg *config.Config, verbose bool) (*Proxy, error) {
	hostAddr := HostSessionBusAddress()
	if hostAddr == "" {
		if verbose {
			fmt.Fprintf(os.Stderr, "[verbose] No session D-Bus socket found on host\n")
		}
		return nil, nil
	}

	hostSock := HostSessionBusSocketPath()
	if hostSock == "" {
		return nil, nil
	}
	if fi, err := os.Stat(hostSock); err != nil || (fi.Mode()&os.ModeSocket == 0 && fi.IsDir()) {
		if verbose {
			fmt.Fprintf(os.Stderr, "[verbose] Host D-Bus socket %s does not exist or is invalid: %v\n", hostSock, err)
		}
		return nil, nil
	}

	destPath, destDir := SandboxDestinationPaths()

	hasProxy := util.CommandExists("xdg-dbus-proxy")
	if !hasProxy {
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

	os.MkdirAll("/tmp/bws", 0700)
	tmpDir, err := os.MkdirTemp("/tmp/bws", "dbus_proxy_")
	if err != nil {
		tmpDir, err = os.MkdirTemp("", "bws_dbus_proxy_")
		if err != nil {
			return nil, fmt.Errorf("creating dbus proxy directory: %w", err)
		}
	}

	proxySock := filepath.Join(tmpDir, "bus")

	talkPolicies := DefaultTalkPolicies
	if cfg != nil && cfg.Features != nil && len(cfg.Features.DBusTalk) > 0 {
		talkPolicies = cfg.Features.DBusTalk
	}

	proxyArgs := []string{hostAddr, proxySock, "--filter"}
	for _, talk := range talkPolicies {
		proxyArgs = append(proxyArgs, "--talk="+talk)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] Launching D-Bus proxy: xdg-dbus-proxy %s\n", strings.Join(proxyArgs, " "))
	}

	cmd := exec.Command("xdg-dbus-proxy", proxyArgs...)
	if err := cmd.Start(); err != nil {
		if err := os.RemoveAll(tmpDir); err != nil {
			// explicitly ignored
		}
		return nil, fmt.Errorf("starting xdg-dbus-proxy: %w", err)
	}

	// Poll until the proxy creates the listening socket (up to 1 second)
	ready := false
	for i := 0; i < 100; i++ {
		time.Sleep(10 * time.Millisecond)
		if fi, err := os.Stat(proxySock); err == nil && (fi.Mode()&os.ModeSocket != 0 || !fi.IsDir()) {
			ready = true
			break
		}
		if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
			break
		}
	}

	if !ready {
		if cmd.Process != nil {
			if err := cmd.Process.Kill(); err != nil {
				// explicitly ignored
			}
			if err := cmd.Wait(); err != nil {
				// explicitly ignored
			}
		}
		if err := os.RemoveAll(tmpDir); err != nil {
			// explicitly ignored
		}
		return nil, fmt.Errorf("xdg-dbus-proxy failed to initialize socket at %s", proxySock)
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
