package learn

import (
	"fmt"
	"path/filepath"
	"strings"
)

var sensitivePathPrefixes = []string{
	"/",
	"/etc",
	"/usr",
	"/bin",
	"/sbin",
	"/lib",
	"/lib64",
	"/lib32",
	"/libx32",
	"/sys",
	"/proc",
	"/dev",
	"~/.ssh",
	"~/.gnupg",
	"~/.aws",
	"~/.azure",
	"~/.config/gcloud",
	"~/.password-store",
	"~/.vault-token",
}

// IsSensitivePath checks if a normalized or raw path targets a security-sensitive location.
func IsSensitivePath(p, homeDir string) bool {
	norm := NormalizePath(p, homeDir)
	if norm == "" || norm == "/" {
		return true
	}

	for _, sensitive := range sensitivePathPrefixes {
		if sensitive == "/" {
			continue
		}
		if norm == sensitive || strings.HasPrefix(norm, sensitive+"/") {
			return true
		}
	}

	// Also check raw absolute paths against expanded home
	if homeDir != "" {
		cleanHome := filepath.Clean(homeDir)
		for _, sensitive := range sensitivePathPrefixes {
			var expanded string
			if sensitive == "~" {
				expanded = cleanHome
			} else if strings.HasPrefix(sensitive, "~/") {
				expanded = filepath.Join(cleanHome, sensitive[2:])
			} else {
				expanded = sensitive
			}
			cleanP := filepath.Clean(p)
			if cleanP == expanded || strings.HasPrefix(cleanP, expanded+"/") {
				return true
			}
		}
	}

	return false
}

// FilterSensitiveWrites removes sensitive paths from RW candidates and returns warnings.
func FilterSensitiveWrites(bindsRW []string, homeDir string) ([]string, []string) {
	var safeRW []string
	var alerts []string

	for _, rw := range bindsRW {
		if IsSensitivePath(rw, homeDir) {
			alerts = append(alerts, fmt.Sprintf("[SECURITY WARNING] Write access detected to sensitive path: %s (will not auto-add to binds_rw)", rw))
		} else {
			safeRW = append(safeRW, rw)
		}
	}

	return safeRW, alerts
}
