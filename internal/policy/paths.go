package policy

import "bws/internal/config"

// WorkspaceToken marks a path relative to the selected project root.
const WorkspaceToken = config.WorkspaceToken

// ExpandWorkspace expands project references after selecting the workspace root.
func ExpandWorkspace(cfg *config.Config, root string) { config.ExpandWorkspace(cfg, root) }
