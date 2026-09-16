package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bws/internal/config"
	"bws/internal/detect"
	"bws/internal/profile"
)

// SuggestionReport includes workspace evidence and ranked candidates.
type SuggestionReport struct {
	Workspace   string               `json:"workspace"`
	Evidence    []detect.Entry       `json:"entries"`
	Suggestions []profile.Suggestion `json:"suggestions"`
}

// SuggestProfiles collects candidates without writing policy or granting trust.
func SuggestProfiles(dir string, compound bool) (*SuggestionReport, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	root, _ := config.FindWorkspaceRoot(abs)
	evidence, err := detect.Scan(root)
	if err != nil {
		return nil, err
	}
	registry, err := profile.LoadRegistry(root)
	if err != nil {
		return nil, err
	}
	matches, err := profile.Suggestions(evidence, registry, compound)
	if err != nil {
		return nil, err
	}
	return &SuggestionReport{evidence.Root, evidence.Entries, matches}, nil
}

// HandleProfileSuggest prints explanations or machine-readable suggestions.
func HandleProfileSuggest(dir string, jsonOutput, compound bool) error {
	report, err := SuggestProfiles(dir, compound)
	if err != nil {
		return err
	}
	if jsonOutput {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("Workspace: %s\n", report.Workspace)
	if len(report.Suggestions) == 0 {
		fmt.Println("No matching profiles. Use 'bws init --profile <name>' to choose explicitly.")
		return nil
	}
	for _, s := range report.Suggestions {
		fmt.Printf("  %s [%s, %s]: %s\n", s.Name, s.Source, s.Kind, strings.Join(s.Evidence, ", "))
		if len(s.Requires) > 0 {
			fmt.Printf("    Includes: %s\n", strings.Join(s.Requires, ", "))
		}
	}
	fmt.Println("Matching describes project contents, not approval or intent. Tied candidates require a choice.")
	return nil
}
