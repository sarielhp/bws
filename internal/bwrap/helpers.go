package bwrap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bws/internal/config"
	"bws/internal/ssh"
	"bws/internal/util"
)

func addSSHArgs(cfg *config.Config, sandboxDir string, args *[]string, dryRun bool) {
	var sshKeys []string
	if cfg.Features != nil {
		sshKeys = cfg.Features.SSHKeys
	}
	var sshAuthSock string

	if dryRun {
		sock := os.Getenv("SSH_AUTH_SOCK")
		if sock != "" {
			if fi, err := os.Stat(sock); err == nil && fi.Mode()&os.ModeSocket != 0 {
				sshAuthSock = sock
			}
		}
	} else {
		if (len(sshKeys) == 0) && config.GetBool(cfg, func(c *config.Config) *bool {
			if c.Features == nil {
				return nil
			}
			return c.Features.AutoRepoDeployKey
		}, true) && util.CommandExists("ssh-agent") && util.CommandExists("ssh-add") {
			if key := ssh.GetAutoDeployKey(); key != "" {
				sshKeys = []string{key}
			}
		}
		sshAuthSock = ssh.EnsureAgent(sshKeys)
	}

	hostSSHDir := filepath.Join(util.HomeDir(), ".ssh")

	if sshAuthSock != "" {
		if fi, err := os.Stat(sshAuthSock); err == nil && fi.Mode()&os.ModeSocket != 0 {
			*args = append(*args, "--bind", sshAuthSock, sshAuthSock)
			*args = append(*args, "--setenv", "SSH_AUTH_SOCK", sshAuthSock)

			if fi, err := os.Stat(filepath.Join(hostSSHDir, "config")); err == nil && !fi.IsDir() {
				*args = append(*args, "--ro-bind", filepath.Join(hostSSHDir, "config"),
					filepath.Join(util.HomeDir(), ".ssh", "config"))
			}
			if fi, err := os.Stat(filepath.Join(hostSSHDir, "known_hosts")); err == nil && !fi.IsDir() {
				*args = append(*args, "--ro-bind", filepath.Join(hostSSHDir, "known_hosts"),
					filepath.Join(util.HomeDir(), ".ssh", "known_hosts"))
			}
		}
	} else if fi, err := os.Stat(hostSSHDir); err == nil && fi.IsDir() {
		if !dryRun {
			os.MkdirAll(filepath.Join(sandboxDir, ".ssh"), 0755)
		}
		for _, f := range []string{"config", "known_hosts"} {
			hostF := filepath.Join(hostSSHDir, f)
			if fi, err := os.Stat(hostF); err == nil && !fi.IsDir() {
				alreadyBound := false
				for _, a := range *args {
					if a == hostF {
						alreadyBound = true
						break
					}
				}
				if !alreadyBound {
					*args = append(*args, "--ro-bind", hostF, filepath.Join(util.HomeDir(), ".ssh", f))
				}
			}
		}
	}
}

func addX11Args(args *[]string) {
	display := os.Getenv("DISPLAY")
	if display == "" {
		return
	}
	*args = append(*args, "--setenv", "DISPLAY", display)

	xauth := os.Getenv("XAUTHORITY")
	if xauth == "" {
		xauth = filepath.Join(util.HomeDir(), ".Xauthority")
	}
	if fi, err := os.Stat(xauth); err == nil && !fi.IsDir() {
		*args = append(*args, "--ro-bind", xauth, filepath.Join(util.HomeDir(), ".Xauthority"))
		*args = append(*args, "--setenv", "XAUTHORITY", filepath.Join(util.HomeDir(), ".Xauthority"))
	}

	*args = append(*args, "--setenv", "NO_AT_SPI", "1")
}

func addWSLArgs(args *[]string) {
	wslInterop := os.Getenv("WSL_INTEROP")
	isWSL := (wslInterop != "") || dirExists("/run/WSL") || fileExists("/proc/sys/fs/binfmt_misc/WSLInterop")

	if isWSL {
		*args = append(*args, "--ro-bind-try", "/init", "/init")
		*args = append(*args, "--bind-try", "/run/WSL", "/run/WSL")
		if wslInterop != "" {
			*args = append(*args, "--setenv", "WSL_INTEROP", wslInterop)
		}
		if fileExists("/mnt/c/Windows/System32/clip.exe") {
			*args = append(*args, "--ro-bind-try", "/mnt/c/Windows/System32/clip.exe", "/mnt/c/Windows/System32/clip.exe")
		}
	}
}

func addEtcAutoBindArgs(args *[]string) {
	entries, err := os.ReadDir("/etc")
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if name != "resolv.conf" && name != "gai.conf" && name != "hosts" {
			path := filepath.Join("/etc", name)
			*args = append(*args, "--ro-bind-try", path, path)
		}
	}
}

func addOptBind(args *[]string) {
	if fi, err := os.Stat("/opt"); err == nil && fi.IsDir() {
		alreadyBound := false
		for _, a := range *args {
			if a == "/opt" {
				alreadyBound = true
				break
			}
		}
		if !alreadyBound {
			*args = append(*args, "--ro-bind-try", "/opt", "/opt")
			if realPath, err := filepath.EvalSymlinks("/opt"); err == nil && realPath != "/opt" {
				if fi, err := os.Stat(realPath); err == nil && fi.IsDir() {
					alreadyBound = false
					for _, a := range *args {
						if a == realPath {
							alreadyBound = true
							break
						}
					}
					if !alreadyBound {
						*args = append(*args, "--ro-bind-try", realPath, realPath)
					}
				}
			}
		}
	}
}

func addQuartoBind(args *[]string) {
	if !util.CommandExists("quarto") {
		return
	}
	quartoPath, err := execLookPath("quarto")
	if err != nil {
		return
	}
	realPath, err := filepath.EvalSymlinks(quartoPath)
	if err != nil {
		realPath = quartoPath
	}
	binDir := filepath.Dir(realPath)
	rootDir := filepath.Dir(binDir)

	if fi, err := os.Stat(rootDir); err == nil && fi.IsDir() {
		alreadyBound := false
		for _, a := range *args {
			if a == rootDir {
				alreadyBound = true
				break
			}
		}
		if !alreadyBound {
			*args = append(*args, "--ro-bind-try", rootDir, rootDir)
		}
	}
	if fi, err := os.Stat(binDir); err == nil && fi.IsDir() {
		alreadyBound := false
		for _, a := range *args {
			if a == binDir {
				alreadyBound = true
				break
			}
		}
		if !alreadyBound {
			*args = append(*args, "--ro-bind-try", binDir, binDir)
		}
	}
}

func dirExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

func execLookPath(file string) (string, error) {
	if strings.Contains(file, "/") {
		if fi, err := os.Stat(file); err == nil && !fi.IsDir() && fi.Mode()&0111 != 0 {
			return file, nil
		}
		return "", fmt.Errorf("not found")
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		p := filepath.Join(dir, file)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() && fi.Mode()&0111 != 0 {
			return p, nil
		}
	}
	return "", fmt.Errorf("not found")
}
