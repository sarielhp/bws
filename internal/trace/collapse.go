package trace

import (
	"path/filepath"
	"sort"
	"strings"
)

// NormalizePath normalizes a raw file path and converts host home directory to ~/
func NormalizePath(rawPath, homeDir string) string {
	clean := filepath.Clean(strings.TrimSpace(rawPath))
	if clean == "" || clean == "." {
		return ""
	}

	if homeDir != "" {
		cleanHome := filepath.Clean(homeDir)
		if clean == cleanHome {
			return "~"
		}
		if strings.HasPrefix(clean, cleanHome+"/") {
			return "~/" + strings.TrimPrefix(clean, cleanHome+"/")
		}
	}

	return clean
}

// CollapsePath collapses granular subpaths into clean root tool/config directories.
func CollapsePath(normalized string) string {
	if normalized == "" || normalized == "~" || normalized == "/" {
		return normalized
	}

	parts := strings.Split(normalized, "/")

	// Handle ~/ paths
	if parts[0] == "~" && len(parts) > 1 {
		// XDG Standard directories: ~/.config/app/..., ~/.cache/app/..., ~/.local/share/app/..., ~/.local/state/app/...
		if (parts[1] == ".config" || parts[1] == ".cache") && len(parts) >= 3 {
			return strings.Join(parts[:3], "/")
		}
		if parts[1] == ".local" && len(parts) >= 4 && (parts[2] == "share" || parts[2] == "state") {
			return strings.Join(parts[:4], "/")
		}
		if parts[1] == ".local" && len(parts) >= 3 && parts[2] == "bin" {
			return strings.Join(parts[:3], "/")
		}

		// Tool-specific dot-directories: ~/.gemini/..., ~/.cargo/..., ~/.npm/..., ~/.docker/...
		if strings.HasPrefix(parts[1], ".") && len(parts) >= 2 {
			// If it's a directory access, collapse to the dot-directory
			if len(parts) > 2 {
				return strings.Join(parts[:2], "/")
			}
			return normalized
		}

		// Top-level non-dot directories under home: ~/go/..., ~/bin/..., ~/opt/...
		if len(parts) >= 3 {
			return strings.Join(parts[:2], "/")
		}
		return normalized
	}

	// Handle system paths outside home: /opt/app/..., /var/log/app/..., /etc/app/...
	if len(parts) >= 4 && (parts[1] == "opt" || parts[1] == "etc" || (parts[1] == "var" && parts[2] == "log")) {
		if parts[1] == "var" && parts[2] == "log" {
			return strings.Join(parts[:4], "/")
		}
		return strings.Join(parts[:3], "/")
	}

	return normalized
}

// CollapseAndClassify aggregates raw file accesses, collapses subpaths, and classifies
// paths into mutually exclusive Read-Write (binds_rw) and Read-Only (binds_ro) slices.
func CollapseAndClassify(accesses map[string]AccessMode, homeDir string) ([]string, []string) {
	collapsedModes := make(map[string]AccessMode)

	for rawPath, mode := range accesses {
		norm := NormalizePath(rawPath, homeDir)
		if norm == "" || norm == "~" || norm == "/" {
			continue
		}
		collapsed := CollapsePath(norm)
		if collapsed == "" || collapsed == "~" || collapsed == "/" {
			continue
		}
		collapsedModes[collapsed] |= mode
	}

	rwMap := make(map[string]bool)
	roMap := make(map[string]bool)

	for path, mode := range collapsedModes {
		if mode&AccessWrite != 0 {
			rwMap[path] = true
		} else if mode&AccessRead != 0 {
			roMap[path] = true
		}
	}

	// Ensure strict mutual exclusivity: RW takes precedence over RO
	for path := range rwMap {
		delete(roMap, path)
	}

	// Remove redundant children when parent is already mounted
	rwMap = pruneRedundantChildren(rwMap)
	roMap = pruneRedundantChildren(roMap)

	// Additional cross-check: if parent is in RW, remove child from RO
	for roPath := range roMap {
		for rwPath := range rwMap {
			if isChildPath(roPath, rwPath) {
				delete(roMap, roPath)
				break
			}
		}
	}

	bindsRW := mapToSortedSlice(rwMap)
	bindsRO := mapToSortedSlice(roMap)

	return bindsRW, bindsRO
}

func isChildPath(child, parent string) bool {
	if child == parent {
		return true
	}
	return strings.HasPrefix(child, parent+"/")
}

func pruneRedundantChildren(paths map[string]bool) map[string]bool {
	result := make(map[string]bool)
	for path := range paths {
		isRedundant := false
		for other := range paths {
			if path != other && isChildPath(path, other) {
				isRedundant = true
				break
			}
		}
		if !isRedundant {
			result[path] = true
		}
	}
	return result
}

func mapToSortedSlice(m map[string]bool) []string {
	res := make([]string, 0, len(m))
	for k := range m {
		res = append(res, k)
	}
	sort.Strings(res)
	return res
}
