package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bws/internal/config"
	"bws/internal/profile"
	"bws/internal/util"
)

// HandleProfileSave snapshots the current workspace configuration as a reusable profile.
func HandleProfileSave(name, description string, global, local, force bool) {
	if !global && !local {
		global = true
	}

	var targetDir string
	if global {
		targetDir = profile.GlobalProfilesDir()
	} else {
		cwd, _ := os.Getwd()
		targetDir = profile.LocalProfilesDir(cwd)
	}

	targetFile := filepath.Join(targetDir, name+".json")
	if !force {
		if _, err := os.Stat(targetFile); err == nil {
			fmt.Fprintf(os.Stderr, "Error: Profile '%s' already exists (%s). Use -f / --force to overwrite.\n", name, targetFile)
			os.Exit(1)
		}
	}

	localConfig := config.LocalPath()
	if _, err := os.Stat(localConfig); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: No local workspace configuration found (.bws/config.jsonc).\n")
		os.Exit(1)
	}

	cfg, err := config.LoadFile(localConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading workspace configuration: %v\n", err)
		os.Exit(1)
	}

	homeDir := util.HomeDir()
	sanitize := func(p string) string {
		if p == homeDir {
			return config.HomeToken
		}
		if strings.HasPrefix(p, homeDir+"/") {
			return config.HomeToken + strings.TrimPrefix(p, homeDir)
		}
		return p
	}

	if description == "" {
		cwd, _ := os.Getwd()
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		description = fmt.Sprintf("Environment snapshot from %s (%s)", cwd, timestamp)
	}

	var paths []string
	for _, pt := range cfg.Path {
		paths = append(paths, sanitize(pt))
	}

	var env map[string]string
	if len(cfg.Env) > 0 {
		env = make(map[string]string, len(cfg.Env))
		for k, v := range cfg.Env {
			env[k] = sanitize(v)
		}
	}

	var copyList []string
	for _, cp := range cfg.Copy {
		copyList = append(copyList, sanitize(cp))
	}

	var rwBinds [][]string
	for _, b := range cfg.BindsRW {
		host := sanitize(b.Host)
		sandbox := sanitize(b.Sandbox)
		if sandbox == "" {
			sandbox = host
		}
		rwBinds = append(rwBinds, []string{host, sandbox})
	}

	var roBinds [][]string
	for _, b := range cfg.BindsRO {
		host := sanitize(b.Host)
		sandbox := sanitize(b.Sandbox)
		if sandbox == "" {
			sandbox = host
		}
		roBinds = append(roBinds, []string{host, sandbox})
	}

	p := profile.Profile{
		Name:        name,
		Description: description,
		Requires:    cfg.Profiles,
		Features:    cfg.Features,
		Path:        paths,
		Env:         env,
		PassEnv:     cfg.PassEnv,
		Mask:        cfg.Mask,
		Copy:        copyList,
		BindsRW:     rwBinds,
		BindsRO:     roBinds,
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating profile directory: %v\n", err)
		os.Exit(1)
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error serializing profile: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(targetFile, append(data, '\n'), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing profile to %s: %v\n", targetFile, err)
		os.Exit(1)
	}

	label := "global"
	if local {
		label = "local"
	}
	fmt.Printf("Saved environment snapshot as %s profile '%s' (%s).\n", label, name, targetFile)
	fmt.Printf("Enable in any workspace with: bws add %s\n", name)
}
