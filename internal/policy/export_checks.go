package policy

import (
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"

	"bws/internal/config"
	"bws/internal/profile"
)

func runtimeEnv(key string) bool {
	return strings.HasPrefix(key, "BWS_") || key == "SSH_AUTH_SOCK" || key == "DBUS_SESSION_BUS_ADDRESS"
}

// InspectExport reports portability of referenced capabilities without copying data.
func InspectExport(p *profile.Profile, cfg *config.Config, root string) (*ExportPlan, error) {
	if err := checkExportEnvironment(cfg); err != nil {
		return nil, err
	}
	return &ExportPlan{Profile: p, MachinePaths: machinePaths(cfg, root)}, nil
}

// SensitiveEnv reports names that must not be exported as literal credentials.
func SensitiveEnv(key string) bool {
	upper := strings.ToUpper(key)
	for _, s := range []string{"TOKEN", "SECRET", "PASSWORD", "API_KEY", "PRIVATE_KEY", "CREDENTIAL", "AUTHORIZATION"} {
		if strings.Contains(upper, s) {
			return true
		}
	}
	return false
}

func checkExportEnvironment(c *config.Config) error {
	var keys []string
	for k, v := range c.Env {
		if SensitiveEnv(k) && v != "@@PASS@@" && v != "@@HOST@@" && v != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	if len(keys) != 0 {
		return fmt.Errorf("refusing to export literal credentials for %s; remove them or use @@PASS@@ in the source configuration", strings.Join(keys, ", "))
	}
	return nil
}

func unsupportedSettings(c *config.Config) []string {
	var fields []string
	if c.SandboxPath != "" {
		fields = append(fields, "sandbox_path")
	}
	if c.ModelsJSONPath != "" {
		fields = append(fields, "models_json_path")
	}
	if c.TmuxSessionName != "" && c.TmuxSessionName != "bwrap-dev" {
		fields = append(fields, "tmux_session_name")
	}
	if c.MaxFileCount != 0 && c.MaxFileCount != 1000 {
		fields = append(fields, "max_file_count")
	}
	if c.Features != nil && c.Features.AutoInit != "" {
		fields = append(fields, "features.auto_init")
	}
	defaults, err := config.Parse([]byte(config.DefaultConfigTemplate), "defaults")
	if err == nil && !reflect.DeepEqual(c.System, defaults.System) {
		fields = append(fields, "system")
	}
	return fields
}

func machinePaths(c *config.Config, root string) []string {
	values := append(append(append([]string{}, c.Path...), c.Copy...), c.Mask...)
	for _, b := range append(append([]config.BindEntry{}, c.BindsRW...), c.BindsRO...) {
		values = append(values, b.Host, b.Sandbox)
	}
	if c.Features != nil {
		values = append(values, c.Features.SSHKeys...)
	}
	for _, v := range c.Env {
		values = append(values, v)
	}
	result := []string{}
	for _, value := range values {
		normalized := portablePath(value, root)
		if !filepath.IsAbs(normalized) {
			continue
		}
		standard := false
		for _, prefix := range []string{"/usr", "/bin", "/sbin", "/lib", "/lib32", "/lib64", "/libx32", "/etc", "/run", "/dev", "/proc", "/sys", "/tmp", "/var/lib/texmf", "/var/cache/fontconfig"} {
			if normalized == prefix || strings.HasPrefix(normalized, prefix+"/") {
				standard = true
				break
			}
		}
		if !standard && !slices.Contains(result, normalized) {
			result = append(result, normalized)
		}
	}
	sort.Strings(result)
	return result
}
