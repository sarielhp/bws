package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bws/internal/config"
	"bws/internal/policy"
)

// InitOptions selects a project setup without inferring user intent from filenames.
type InitOptions struct {
	TargetDir string
	Profiles  []string
	Preset    string
	OpenCode  bool
	Basic     bool
	Force     bool
	DryRun    bool
	Yes       bool
	Flags     policy.Flags
}

// HandleInit keeps the historical Go entry point as an explicit basic-init adapter.
func HandleInit(targetDir string, force, dryRun, noSSH, opencode bool, preset string, profiles []string) error {
	return HandleInitOptions(InitOptions{TargetDir: targetDir, Force: force, DryRun: dryRun, OpenCode: opencode, Preset: preset, Profiles: profiles, Basic: true, Flags: policy.Flags{NoSSH: noSSH}})
}

// HandleInitDev preserves the init-dev handler alias.
func HandleInitDev(targetDir string, force, dryRun, noSSH, opencode bool, preset string, profiles []string) error {
	return HandleInit(targetDir, force, dryRun, noSSH, opencode, preset, profiles)
}

// HandleInitOptions previews and applies a reviewed initialization plan.
func HandleInitOptions(opts InitOptions) error {
	root, before, err := initDestination(opts.TargetDir)
	if err != nil {
		return err
	}
	if before != nil && !opts.Force {
		fmt.Printf("Workspace already initialized: %s\nUnchanged. Use --force with an explicit selection to replace it.\n", root)
		return nil
	}
	names, err := selectInitProfiles(root, opts)
	if err != nil {
		return err
	}
	plan, err := BuildInitPlan(root, names, opts.Flags)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Workspace: %s\nSelected profiles: %s\n", root, strings.Join(names, ", "))
	PrintPolicySummary(os.Stderr, plan.Effective)
	if opts.DryRun {
		_, err = os.Stdout.Write(plan.Data)
		return err
	}
	if !opts.Yes && IsInteractiveTTY(int(os.Stdin.Fd())) {
		if err := confirmPolicy(os.Stdin, os.Stderr, "Initialize with these permissions?"); err != nil {
			return err
		}
	}
	path := filepath.Join(root, ".bws", "config.jsonc")
	if before != nil {
		backup := path + ".bak"
		prior, err := config.PolicyBytes(backup)
		if err != nil {
			return err
		}
		if err := config.AtomicPolicyWrite(backup, before, prior); err != nil {
			return err
		}
		fmt.Printf("Backed up existing configuration to: %s\n", backup)
	}
	if err := config.AtomicPolicyWrite(path, plan.Data, before); err != nil {
		return err
	}
	fmt.Printf("Initialized development sandbox configuration: %s\n", path)
	return nil
}

func initDestination(dir string) (string, []byte, error) {
	if dir == "" {
		dir = "."
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", nil, err
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return "", nil, err
	}
	if !fi.IsDir() {
		return "", nil, fmt.Errorf("target path is not a directory: %s", abs)
	}
	root, path := config.FindWorkspaceRoot(abs)
	if err := config.ValidateWorkspace(root, 1000, true); err != nil {
		return "", nil, err
	}
	before, err := config.PolicyBytes(path)
	if err != nil {
		return "", nil, err
	}
	if before != nil {
		if _, err := config.ReadTrustedFile(path); err != nil {
			return "", nil, err
		}
		if filepath.Base(path) != "config.jsonc" || filepath.Base(filepath.Dir(path)) != ".bws" {
			return "", nil, fmt.Errorf("legacy configuration %s must be migrated to .bws/config.jsonc before reinitialization", path)
		}
	}
	return root, before, nil
}
