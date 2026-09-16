package config

import (
	"bws/internal/util"
	"strings"
)

// WorkspaceToken is expanded only after the workspace root has been selected.
const WorkspaceToken = "@@WORKSPACE@@"

// ExpandWorkspace expands workspace references in path-bearing declarations.
func ExpandWorkspace(cfg *Config, root string) {
	replace := func(s string) string {
		return strings.ReplaceAll(strings.ReplaceAll(s, WorkspaceToken, root), HomeToken, util.HomeDir())
	}
	for i, v := range cfg.Path {
		cfg.Path[i] = replace(v)
	}
	for i, v := range cfg.Mask {
		cfg.Mask[i] = replace(v)
	}
	for i, v := range cfg.Copy {
		cfg.Copy[i] = replace(v)
	}
	for k, v := range cfg.Env {
		cfg.Env[k] = replace(v)
	}
	for i, v := range cfg.BindsRW {
		cfg.BindsRW[i] = BindEntry{Host: replace(v.Host), Sandbox: replace(v.Sandbox)}
	}
	for i, v := range cfg.BindsRO {
		cfg.BindsRO[i] = BindEntry{Host: replace(v.Host), Sandbox: replace(v.Sandbox)}
	}
	if cfg.Features != nil {
		for i, v := range cfg.Features.SSHKeys {
			cfg.Features.SSHKeys[i] = replace(v)
		}
	}
}
