package cli

import (
	"bws/internal/config"
	"bws/internal/profile"
	"bws/internal/util"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	"golang.org/x/term"
)

func HandleProfileTest(name string, verbose bool) error {
	cwd, _ := os.Getwd()
	registry, err := profile.LoadRegistry(cwd)
	if err != nil {
		return err
	}

	ctx := profile.DetectMatchContext()
	resolved, err := profile.ResolveProfile(name, registry, ctx)
	if err != nil {
		return err
	}

	globalPath := config.GlobalPath()
	baseCfg, err := config.LoadFile(globalPath)
	if err != nil {
		baseCfg = &config.Config{
			System: &config.SystemConfig{
				ShareNet: boolPtr(true),
			},
		}
	}
	localPath := config.LocalPath()
	if fi, err := os.Stat(localPath); err == nil && !fi.IsDir() {
		if localCfg, err := config.LoadFile(localPath); err == nil {
			baseCfg = config.Merge(baseCfg, localCfg)
		}
	}

	fmt.Printf("Testing profile %s in sandbox:\n", ColorProfile(name))
	results, err := profile.RunProfileTests(baseCfg, cwd, resolved, verbose)
	if err != nil {
		return err
	}

	passedCount := 0
	skippedCount := 0
	failedCount := 0

	for _, r := range results {
		switch r.Status {
		case "passed":
			passedCount++
			fmt.Printf("  ✓ %-35s (%s)\n", r.Name, r.Duration.Round(time.Millisecond))
		case "skipped":
			skippedCount++
			fmt.Printf("  ⊘ %-35s [skipped: %s]\n", r.Name, r.Output)
		case "failed":
			failedCount++
			fmt.Printf("  ✗ %-35s [FAILED]\n", r.Name)
			if r.Output != "" {
				fmt.Printf("    Output:\n      %s\n", strings.ReplaceAll(r.Output, "\n", "\n      "))
			}
		}
	}

	fmt.Println()
	if failedCount > 0 {
		return fmt.Errorf("profile %q test failed: %d passed, %d skipped, %d failed", name, passedCount, skippedCount, failedCount)
	}

	fmt.Printf("Summary: %d passed, %d skipped. Everything is fine.\n", passedCount, skippedCount)
	return nil
}

// HandleProfileSearch searches local profiles, host tools, and Homebrew formula registry.
func HandleProfileSearch(query string) error {
	cleanQ := strings.ToLower(strings.TrimSpace(query))
	if cleanQ == "" {
		return fmt.Errorf("search query cannot be empty")
	}

	cwd, _ := os.Getwd()
	registry, _ := profile.LoadRegistry(cwd)

	fmt.Printf("Searching for %q:\n\n", query)

	// 1. Local / Embedded Profiles
	var matchedProfiles []*profile.Profile
	seen := make(map[string]bool)
	for name, p := range registry {
		matched := strings.Contains(strings.ToLower(name), cleanQ) || strings.Contains(strings.ToLower(p.Description), cleanQ)
		if !matched {
			for _, a := range p.Aliases {
				if strings.Contains(strings.ToLower(a), cleanQ) {
					matched = true
					break
				}
			}
		}
		if matched && !seen[p.Name] {
			seen[p.Name] = true
			matchedProfiles = append(matchedProfiles, p)
		}
	}
	sort.Slice(matchedProfiles, func(i, j int) bool {
		return matchedProfiles[i].Name < matchedProfiles[j].Name
	})

	dim := color.New(color.FgHiBlack).SprintFunc()
	if len(matchedProfiles) > 0 {
		fmt.Println("Local & Embedded Profiles:")
		for _, p := range matchedProfiles {
			src := p.Source
			if src == "" {
				src = "embedded"
			}
			aliasStr := ""
			if len(p.Aliases) > 0 {
				var coloredAliases []string
				for _, a := range p.Aliases {
					coloredAliases = append(coloredAliases, ColorProfile(a))
				}
				aliasStr = fmt.Sprintf(" (aliases: %s)", strings.Join(coloredAliases, ", "))
			}
			fmt.Printf("  ★ %-12s [%-8s] %s%s\n", ColorProfile(p.Name), dim(src), p.Description, aliasStr)
		}
		fmt.Println()
	}

	// 2. Host Binary check
	if util.CommandExists(cleanQ) {
		fmt.Printf("Host Executable: %q is installed on this system.\n\n", cleanQ)
	}

	// 3. Query Homebrew Index
	client := &http.Client{Timeout: 3 * time.Second}
	hbURL := fmt.Sprintf("https://formulae.brew.sh/api/formula/%s.json", cleanQ)
	if resp, err := client.Get(hbURL); err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var hb struct {
			Name string `json:"name"`
			Desc string `json:"desc"`
		}
		if err := json.Unmarshal(body, &hb); err == nil && hb.Name != "" {
			fmt.Println("Available on Homebrew:")
			fmt.Printf("  • %-12s [Formula]  %s\n", ColorProfile(hb.Name), hb.Desc)
			fmt.Printf("    Run 'bws profile generate %s' to generate a sandbox profile.\n\n", ColorProfile(hb.Name))
		}
	}

	return nil
}

// HandleProfileFetch downloads a profile from GitHub repository or synthesizes it.
func HandleProfileFetch(name string, global, local bool) error {
	cleanName := strings.ToLower(strings.TrimSpace(name))
	if cleanName == "" {
		return fmt.Errorf("profile name cannot be empty")
	}

	var targetDir string
	if local {
		cwd, _ := os.Getwd()
		targetDir = profile.LocalProfilesDir(cwd)
	} else {
		targetDir = profile.GlobalProfilesDir()
	}
	os.MkdirAll(targetDir, 0755)

	targetFile := filepath.Join(targetDir, cleanName+".json")

	// Try remote GitHub fetch
	client := &http.Client{Timeout: 5 * time.Second}
	url := fmt.Sprintf("https://raw.githubusercontent.com/sarielhp/bws/main/profiles/%s.json", cleanName)

	resp, err := client.Get(url)
	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err == nil {
			var p profile.Profile
			if err := json.Unmarshal(data, &p); err == nil {
				if err := os.WriteFile(targetFile, data, 0644); err == nil {
					fmt.Printf("Fetched profile %s from GitHub -> %s\n", ColorProfile(cleanName), targetFile)
					return nil
				}
			}
		}
	}

	// Fall back to synthesis
	fmt.Printf("Profile %s not found in remote catalog; attempting synthesis...\n", ColorProfile(cleanName))
	return HandleProfileNew(cleanName, global, local)
}

// HandleProfileUpdate updates all locally installed global profiles from the remote repository.
func HandleProfileUpdate() error {
	dir := profile.GlobalProfilesDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading profiles directory: %w", err)
	}

	updated := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		pName := strings.TrimSuffix(e.Name(), ".json")
		if err := HandleProfileFetch(pName, true, false); err == nil {
			updated++
		}
	}

	fmt.Printf("Profile update complete (%d global profiles updated).\n", updated)
	return nil
}

// HandleProfileCurrent displays the installed profiles applied to the current workspace in exact order.
func HandleProfileCurrent() error {
	return HandleStatusShort()
}

func getTerminalWidth() int {
	fd := int(os.Stdout.Fd())
	if term.IsTerminal(fd) {
		if w, _, err := term.GetSize(fd); err == nil && w > 40 {
			return w
		}
	}
	return 80
}

func wrapText(text string, indent, maxWidth int) string {
	if maxWidth <= indent+20 {
		maxWidth = 80
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}

	var lines []string
	var curLine strings.Builder
	curWidth := 0

	for _, w := range words {
		wLen := len(w)
		if curWidth == 0 {
			curLine.WriteString(w)
			curWidth = wLen
		} else if curWidth+1+wLen <= maxWidth-indent {
			curLine.WriteByte(' ')
			curLine.WriteString(w)
			curWidth += 1 + wLen
		} else {
			lines = append(lines, curLine.String())
			curLine.Reset()
			curLine.WriteString(w)
			curWidth = wLen
		}
	}
	if curLine.Len() > 0 {
		lines = append(lines, curLine.String())
	}

	indentStr := strings.Repeat(" ", indent)
	return strings.Join(lines, "\n"+indentStr)
}

func boolPtr(b bool) *bool {
	return &b
}
