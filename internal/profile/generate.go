package profile

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type brewFormulaResponse struct {
	Name         string   `json:"name"`
	Desc         string   `json:"desc"`
	Dependencies []string `json:"dependencies"`
}

// GenerateProfile fetches Homebrew and Firejail intelligence to create a new Profile.
func GenerateProfile(name string) (*Profile, error) {
	cleanName := strings.ToLower(strings.TrimSpace(name))
	if cleanName == "" {
		return nil, fmt.Errorf("profile name cannot be empty")
	}

	p := &Profile{
		Name: cleanName,
	}

	client := &http.Client{Timeout: 5 * time.Second}

	// 1. Query Homebrew Formula API
	hbURL := fmt.Sprintf("https://formulae.brew.sh/api/formula/%s.json", cleanName)
	if resp, err := client.Get(hbURL); err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var hb brewFormulaResponse
		if err := json.Unmarshal(body, &hb); err == nil {
			if hb.Desc != "" {
				p.Description = hb.Desc
			}
			for _, dep := range hb.Dependencies {
				if dep != "" && dep != cleanName {
					p.Requires = append(p.Requires, dep)
				}
			}
		}
	}

	if p.Description == "" {
		p.Description = fmt.Sprintf("%s toolchain and environment", cleanName)
	}

	// 2. Query Firejail Profile Repository
	fjURL := fmt.Sprintf("https://raw.githubusercontent.com/netblue30/firejail/master/etc/%s.profile", cleanName)
	var fjWhitelists []string
	var fjReadOnlys []string
	if resp, err := client.Get(fjURL); err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "#") || line == "" {
				continue
			}
			if strings.HasPrefix(line, "whitelist ") {
				path := strings.TrimSpace(strings.TrimPrefix(line, "whitelist "))
				fjWhitelists = append(fjWhitelists, path)
			} else if strings.HasPrefix(line, "read-only ") {
				path := strings.TrimSpace(strings.TrimPrefix(line, "read-only "))
				fjReadOnlys = append(fjReadOnlys, path)
			}
		}
	}

	seenRW := make(map[string]bool)
	seenRO := make(map[string]bool)

	for _, raw := range fjWhitelists {
		b := convertFirejailPath(raw)
		if len(b) == 2 && !seenRW[b[0]] {
			seenRW[b[0]] = true
			p.BindsRW = append(p.BindsRW, b)
		}
	}

	for _, raw := range fjReadOnlys {
		b := convertFirejailPath(raw)
		if len(b) == 2 && !seenRO[b[0]] {
			seenRO[b[0]] = true
			p.BindsRO = append(p.BindsRO, b)
		}
	}

	// 3. Fallback standard XDG directories if no custom whitelists were found
	if len(p.BindsRW) == 0 {
		xdgPaths := []string{
			fmt.Sprintf("~/.config/%s", cleanName),
			fmt.Sprintf("~/.cache/%s", cleanName),
			fmt.Sprintf("~/.local/share/%s", cleanName),
		}
		for _, raw := range xdgPaths {
			b := convertFirejailPath(raw)
			p.BindsRW = append(p.BindsRW, b)
		}
	}

	// 4. Formulate Tests
	p.Tests = []TestSpec{
		{
			Name: fmt.Sprintf("%s binary version check", cleanName),
			Cmd:  []string{cleanName, "--version"},
			Type: "version",
		},
	}

	// 5. Formulate Detect
	p.Detect = &DetectSpec{
		Files: []string{
			fmt.Sprintf("%s.json", cleanName),
			fmt.Sprintf(".%s", cleanName),
		},
		Globs: []string{
			fmt.Sprintf("*.%s", cleanName),
		},
	}

	return p, nil
}

func convertFirejailPath(raw string) []string {
	clean := strings.Replace(raw, "${HOME}", "~", -1)
	clean = strings.Replace(clean, "$HOME", "~", -1)
	clean = strings.TrimSpace(clean)

	if strings.HasPrefix(clean, "~") {
		sandboxTarget := strings.Replace(clean, "~", "@@HOME@@", 1)
		return []string{clean, sandboxTarget}
	}
	if strings.HasPrefix(clean, "/") {
		return []string{clean, clean}
	}
	return nil
}

// SaveProfile saves a profile to JSON file.
func SaveProfile(p *Profile, targetPath string) error {
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %q: %w", dir, err)
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode profile: %w", err)
	}

	if err := os.WriteFile(targetPath, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("failed to write profile file: %w", err)
	}
	return nil
}
