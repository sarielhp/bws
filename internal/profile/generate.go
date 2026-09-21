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

	"bws/internal/config"
)

type brewFormulaResponse struct {
	Name         string   `json:"name"`
	Desc         string   `json:"desc"`
	Dependencies []string `json:"dependencies"`
}

// GenerateProfile fetches Homebrew and Firejail intelligence to create a new Profile.
// Unknown dependencies from Homebrew formulas that do not exist in the profile registry are excluded.
func GenerateProfile(name string, registry map[string]*Profile) (*Profile, error) {
	cleanName := strings.ToLower(strings.TrimSpace(name))
	if cleanName == "" {
		return nil, fmt.Errorf("profile name cannot be empty")
	}

	p := &Profile{
		Name: cleanName,
	}

	if registry == nil {
		registry, _ = LoadRegistry("")
	}

	client := &http.Client{Timeout: 5 * time.Second}
	fetchHomebrewFormula(client, cleanName, p, registry)

	if p.Description == "" {
		p.Description = fmt.Sprintf("%s toolchain and environment", cleanName)
	}

	fjWhitelists, fjReadOnlys, fjKeepVars := fetchFirejail(client, cleanName)
	populateGeneratedAccess(p, cleanName, fjWhitelists, fjReadOnlys, fjKeepVars)

	// 5. Formulate Tests
	p.Tests = []TestSpec{
		{
			Name: fmt.Sprintf("%s binary version check", cleanName),
			Cmd:  []string{cleanName, "--version"},
			Type: "version",
		},
	}

	// 6. Formulate Detect
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

func fetchHomebrewFormula(client *http.Client, cleanName string, p *Profile, registry map[string]*Profile) {
	hbURL := fmt.Sprintf("https://formulae.brew.sh/api/formula/%s.json", cleanName)
	resp, err := client.Get(hbURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	var hb brewFormulaResponse
	if err := json.Unmarshal(body, &hb); err != nil {
		return
	}
	if hb.Desc != "" {
		p.Description = hb.Desc
	}
	p.Requires = filterValidDependencies(hb.Dependencies, cleanName, registry)
}

func filterValidDependencies(deps []string, selfName string, registry map[string]*Profile) []string {
	if len(deps) == 0 || registry == nil {
		return nil
	}
	seen := make(map[string]bool)
	var valid []string
	for _, raw := range deps {
		dep := strings.TrimSpace(raw)
		if dep == "" || dep == selfName {
			continue
		}
		target := dep
		p, ok := registry[target]
		if !ok && strings.Contains(dep, "@") {
			base := strings.Split(dep, "@")[0]
			p, ok = registry[base]
		}
		if !ok || p == nil {
			continue
		}
		canon := p.Name
		if canon == "" || canon == selfName || seen[canon] {
			continue
		}
		seen[canon] = true
		valid = append(valid, canon)
	}
	return valid
}

func fetchFirejail(client *http.Client, cleanName string) ([]string, []string, []string) {
	// 2. Query Firejail Profile Repository
	fjURL := fmt.Sprintf("https://raw.githubusercontent.com/netblue30/firejail/master/etc/%s.profile", cleanName)
	var fjWhitelists []string
	var fjReadOnlys []string
	var fjKeepVars []string
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
			} else if strings.HasPrefix(line, "keep-var ") {
				varName := strings.TrimSpace(strings.TrimPrefix(line, "keep-var "))
				for _, v := range strings.Fields(varName) {
					fjKeepVars = append(fjKeepVars, v)
				}
			}
		}
	}

	return fjWhitelists, fjReadOnlys, fjKeepVars
}

func populateGeneratedAccess(p *Profile, cleanName string, fjWhitelists, fjReadOnlys, fjKeepVars []string) {
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

	// 3. Environment variable extraction & classification
	seenEnv := make(map[string]bool)
	for _, v := range fjKeepVars {
		cleanVar := strings.TrimSpace(v)
		if cleanVar != "" && !seenEnv[cleanVar] {
			seenEnv[cleanVar] = true
			if isSensitiveEnvVar(cleanVar) {
				// Don't auto-enable sensitive keys in pass_env
				continue
			}
			p.PassEnv = append(p.PassEnv, cleanVar)
		}
	}

	// 4. Fallback standard XDG directories if no custom whitelists were found
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

}

func isSensitiveEnvVar(name string) bool {
	upper := strings.ToUpper(name)
	sensitiveTokens := []string{"KEY", "TOKEN", "SECRET", "PASS", "AUTH", "CREDENTIAL"}
	for _, token := range sensitiveTokens {
		if strings.Contains(upper, token) {
			return true
		}
	}
	return false
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

	if err := config.WriteTrustedFile(targetPath, append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write profile file: %w", err)
	}
	return nil
}
