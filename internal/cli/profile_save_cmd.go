package cli

import (
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

	cfg, err := config.LoadLocalFile(localConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading workspace configuration: %v\n", err)
		os.Exit(1)
	}

	p := profileSnapshot(cfg, name, description)
	if err := profile.SaveProfile(&p, targetFile); err != nil {
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

func profileSnapshot(cfg *config.Config, name, description string) profile.Profile {
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

	return profile.Profile{
		Name:        name,
		Description: description,
		Requires:    cfg.Profiles,
		Features:    cfg.Features,
		Path:        paths,
		Env:         env,
		PassEnv:     cfg.PassEnv,
		Mask:        cfg.Mask,
		Copy:        copyList,
		BindsRW:     snapshotBinds(cfg.BindsRW, sanitize),
		BindsRO:     snapshotBinds(cfg.BindsRO, sanitize),
	}
}

func snapshotBinds(entries []config.BindEntry, sanitize func(string) string) [][]string {
	var binds [][]string
	for _, entry := range entries {
		host, dest := sanitize(entry.Host), sanitize(entry.Sandbox)
		if dest == "" {
			dest = host
		}
		binds = append(binds, []string{host, dest})
	}
	return binds
}
