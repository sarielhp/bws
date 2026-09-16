package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"bws/internal/config"
)

// PrintPolicySummary describes effective grants without displaying secret values.
func PrintPolicySummary(w io.Writer, cfg *config.Config) {
	offline := config.FeatureEnabledDefault(cfg, func(f *config.FeaturesConfig) *bool { return f.NoNet }, false) ||
		config.FeatureEnabledDefault(cfg, func(f *config.FeaturesConfig) *bool { return f.UnshareNet }, false)
	fmt.Fprintf(w, "Effective permissions (including global configuration):\n  Network: %s\n  SSH forwarding: %t\n  X11: %t; D-Bus: %t\n",
		map[bool]string{true: "isolated", false: "shared"}[offline],
		config.FeatureEnabled(cfg, func(f *config.FeaturesConfig) *bool { return f.EnableSSH }),
		config.FeatureEnabled(cfg, func(f *config.FeaturesConfig) *bool { return f.EnableX11 }),
		config.FeatureEnabledDefault(cfg, func(f *config.FeaturesConfig) *bool { return f.EnableDBus }, false))
	for _, b := range cfg.BindsRW {
		dest := b.Sandbox
		if dest == "" {
			dest = b.Host
		}
		fmt.Fprintf(w, "  Writable: %s -> %s\n", b.Host, dest)
	}
	fmt.Fprintf(w, "  Read-only mounts: %d; masks: %d; copy declarations: %d\n", len(cfg.BindsRO), len(cfg.Mask), len(cfg.Copy))
	var keys []string
	for k := range cfg.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Fprintf(w, "  Environment names (values hidden): %s\n", strings.Join(keys, ", "))
	fmt.Fprintf(w, "  Passed environment: %s\n", strings.Join(cfg.PassEnv, ", "))
}

func confirmPolicy(r io.Reader, w io.Writer, prompt string) error {
	fmt.Fprintf(w, "%s [y/N] ", prompt)
	line, err := readPolicyAnswer(r)
	if err != nil {
		return fmt.Errorf("cancelled: %w", err)
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	if answer != "y" && answer != "yes" {
		return fmt.Errorf("cancelled; no changes written")
	}
	return nil
}

func readPolicyAnswer(r io.Reader) (string, error) {
	var line strings.Builder
	var b [1]byte
	for line.Len() < 4096 {
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return "", err
		}
		if b[0] == '\n' {
			return line.String(), nil
		}
		line.WriteByte(b[0])
	}
	return "", fmt.Errorf("input exceeds 4096 bytes")
}

func writeJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}
