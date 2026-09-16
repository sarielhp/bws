package profile

import (
	"bws/internal/detect"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// Suggestion explains a matching profile; rank is evidence strength, not probability.
type Suggestion struct {
	Name     string   `json:"name"`
	Source   string   `json:"source"`
	Kind     string   `json:"kind,omitempty"`
	Requires []string `json:"requires,omitempty"`
	Evidence []string `json:"evidence"`
	Rank     int      `json:"rank"`
}

// Suggestions matches each unique profile and sorts ties by name.
func Suggestions(e *detect.Evidence, registry map[string]*Profile, compound bool) ([]Suggestion, error) {
	result := []Suggestion{}
	seen := map[string]bool{}
	for _, p := range registry {
		if seen[p.Name] || p.Detect == nil || (compound && p.Kind != "compound") {
			continue
		}
		seen[p.Name] = true
		if err := ValidateDetect(p.Detect); err != nil {
			return nil, fmt.Errorf("profile %s: %w", p.Name, err)
		}
		rank, reasons := matchDetect(p.Detect, e)
		if rank == 0 {
			continue
		}
		result = append(result, Suggestion{p.Name, p.Source, p.Kind, p.Requires, reasons, rank})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Rank != result[j].Rank {
			return result[i].Rank > result[j].Rank
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}

// DetectProfiles is the compatibility adapter for basic automatic initialization.
func DetectProfiles(projectDir string, registry map[string]*Profile) []*Profile {
	e, err := detect.Scan(projectDir)
	if err != nil {
		return nil
	}
	matches, err := Suggestions(e, registry, false)
	if err != nil {
		return nil
	}
	var result []*Profile
	for _, m := range matches {
		if registry[m.Name].Kind != "compound" && m.Rank > 1 {
			result = append(result, registry[m.Name])
		}
	}
	return result
}

// ValidateDetect accepts bounded filename predicates, not arbitrary expressions.
func ValidateDetect(d *DetectSpec) error {
	if d == nil {
		return nil
	}
	if len(d.AllOf) > 16 {
		return fmt.Errorf("detect.all_of allows at most 16 groups")
	}
	for _, group := range d.AllOf {
		if len(group.AllOf) != 0 {
			return fmt.Errorf("nested detect.all_of is not supported")
		}
		if len(group.Files)+len(group.Globs)+len(group.DirContains) == 0 {
			return fmt.Errorf("empty detect.all_of group")
		}
		if err := ValidateDetect(&group); err != nil {
			return err
		}
	}
	for _, pattern := range append(append([]string{}, d.Files...), d.Globs...) {
		if pattern == "" || filepath.IsAbs(pattern) || strings.Contains(pattern, "..") {
			return fmt.Errorf("invalid project-relative detection pattern %q", pattern)
		}
		if _, err := filepath.Match(pattern, ""); err != nil {
			return err
		}
	}
	return nil
}

func matchDetect(d *DetectSpec, e *detect.Evidence) (int, []string) {
	rank, reasons := matchAlternatives(d, e)
	hasAny := len(d.Files)+len(d.Globs)+len(d.DirContains) > 0
	if hasAny && rank == 0 {
		return 0, nil
	}
	for _, group := range d.AllOf {
		r, why := matchAlternatives(&group, e)
		if r == 0 {
			return 0, nil
		}
		if rank == 0 || r < rank {
			rank = r
		}
		reasons = append(reasons, why...)
	}
	sort.Strings(reasons)
	return rank, reasons
}

func matchAlternatives(d *DetectSpec, e *detect.Evidence) (int, []string) {
	rank := 0
	reasons := []string{}
	for _, entry := range e.Entries {
		strength := 0
		for _, file := range d.Files {
			if markerMatch(file, entry.Path, false) {
				strength = 4
			}
		}
		for _, glob := range d.Globs {
			if markerMatch(glob, entry.Path, true) && strength == 0 {
				strength = 2
			}
		}
		if strength == 0 {
			continue
		}
		if strings.Contains(entry.Path, "/") {
			strength--
		}
		if strength > rank {
			rank = strength
		}
		reasons = append(reasons, entry.Path)
	}
	for _, hint := range d.DirContains {
		if hint != "" && strings.Contains(strings.ToLower(filepath.Base(e.Root)), strings.ToLower(hint)) {
			if rank == 0 {
				rank = 1
			}
			reasons = append(reasons, "directory-name hint: "+hint)
		}
	}
	return rank, reasons
}

func markerMatch(pattern, path string, glob bool) bool {
	candidate := path
	if !strings.Contains(pattern, "/") {
		candidate = filepath.Base(path)
	}
	if !glob {
		return pattern == candidate
	}
	return matchGlob(pattern, candidate)
}

func matchGlob(pattern, name string) bool {
	matched, err := filepath.Match(pattern, name)
	return err == nil && matched
}

// SuggestedDetect proposes root manifest rules for review when saving.
func SuggestedDetect(e *detect.Evidence) *DetectSpec {
	groups := [][]string{{"go.mod", "go.work"}, {"pyproject.toml", "requirements.txt", "Pipfile", "uv.lock"}, {"Cargo.toml"}, {"package.json"}, {"latexmkrc", ".latexmkrc", "Tectonic.toml"}}
	var all []DetectSpec
	for _, files := range groups {
		found := false
		for _, entry := range e.Entries {
			if strings.Contains(entry.Path, "/") {
				continue
			}
			for _, file := range files {
				if entry.Path == file && !entry.Directory {
					found = true
				}
			}
		}
		if found {
			all = append(all, DetectSpec{Files: files})
		}
	}
	if len(all) == 0 {
		return nil
	}
	if len(all) == 1 {
		return &all[0]
	}
	return &DetectSpec{AllOf: all}
}
