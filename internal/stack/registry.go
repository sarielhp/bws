package stack

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"bws/internal/config"
)

//go:embed embedded_stacks/*.json
var embeddedStacksFS embed.FS

// UserStacksDir returns the directory path for user-saved stacks (~/.config/bws/stacks).
func UserStacksDir() string {
	return filepath.Join(config.ConfigDir(), "stacks")
}

// LoadEmbeddedStacks loads all curated seed stacks embedded in the binary.
func LoadEmbeddedStacks() (map[string]*Stack, error) {
	stacks := make(map[string]*Stack)
	entries, err := fs.ReadDir(embeddedStacksFS, "embedded_stacks")
	if err != nil {
		return nil, fmt.Errorf("reading embedded stacks directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := embeddedStacksFS.ReadFile("embedded_stacks/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("reading embedded stack %s: %w", entry.Name(), err)
		}
		stk, err := Parse(data)
		if err != nil {
			return nil, fmt.Errorf("parsing embedded stack %s: %w", entry.Name(), err)
		}
		stk.Source = "embedded"
		stacks[stk.Name] = stk
	}
	return stacks, nil
}

// LoadUserStacks loads all user-saved stacks from ~/.config/bws/stacks.
func LoadUserStacks() (map[string]*Stack, error) {
	stacks := make(map[string]*Stack)
	dir := UserStacksDir()
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return stacks, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading user stacks directory %s: %w", dir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading user stack file %s: %w", path, err)
		}
		stk, err := Parse(data)
		if err != nil {
			return nil, fmt.Errorf("parsing user stack %s: %w", path, err)
		}
		stk.Source = path
		stacks[stk.Name] = stk
	}
	return stacks, nil
}

// LoadRegistry returns all available stacks, prioritizing user-saved stacks over embedded stacks.
func LoadRegistry() (map[string]*Stack, error) {
	reg, err := LoadEmbeddedStacks()
	if err != nil {
		return nil, err
	}
	userStacks, err := LoadUserStacks()
	if err != nil {
		return nil, err
	}
	for name, stk := range userStacks {
		reg[name] = stk
	}
	return reg, nil
}

// Get retrieves a stack by name, checking user stacks first then embedded stacks.
func Get(name string) (*Stack, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}
	userPath := filepath.Join(UserStacksDir(), name+".json")
	if data, err := os.ReadFile(userPath); err == nil {
		stk, err := Parse(data)
		if err != nil {
			return nil, fmt.Errorf("parsing user stack %s: %w", userPath, err)
		}
		stk.Source = userPath
		return stk, nil
	}
	embeddedData, err := embeddedStacksFS.ReadFile("embedded_stacks/" + name + ".json")
	if err == nil {
		stk, err := Parse(embeddedData)
		if err != nil {
			return nil, fmt.Errorf("parsing embedded stack %s: %w", name, err)
		}
		stk.Source = "embedded"
		return stk, nil
	}
	return nil, fmt.Errorf("stack %q not found", name)
}

// List returns all stacks sorted by Category then Name, optionally filtered by category.
func List(category string) ([]*Stack, error) {
	reg, err := LoadRegistry()
	if err != nil {
		return nil, err
	}
	var result []*Stack
	catFilter := strings.ToLower(strings.TrimSpace(category))
	for _, stk := range reg {
		if catFilter != "" && !strings.Contains(strings.ToLower(stk.Category), catFilter) {
			continue
		}
		result = append(result, stk)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Category != result[j].Category {
			return result[i].Category < result[j].Category
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}

// Search searches stacks matching a case-insensitive query across Name, Title, Description, and Profiles.
func Search(query string) ([]*Stack, error) {
	reg, err := LoadRegistry()
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(strings.TrimSpace(query))
	var matches []*Stack
	for _, stk := range reg {
		if stackMatchesQuery(stk, q) {
			matches = append(matches, stk)
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Name < matches[j].Name
	})
	return matches, nil
}

func stackMatchesQuery(s *Stack, q string) bool {
	if strings.Contains(strings.ToLower(s.Name), q) ||
		strings.Contains(strings.ToLower(s.Title), q) ||
		strings.Contains(strings.ToLower(s.Description), q) ||
		strings.Contains(strings.ToLower(s.Category), q) {
		return true
	}
	for _, p := range s.Profiles {
		if strings.Contains(strings.ToLower(p), q) {
			return true
		}
	}
	return false
}

// SaveUserStack persists a user stack into ~/.config/bws/stacks/<name>.json atomically.
func SaveUserStack(s *Stack) (string, error) {
	if err := Validate(s); err != nil {
		return "", err
	}
	dir := UserStacksDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating user stacks directory: %w", err)
	}
	target := filepath.Join(dir, s.Name+".json")
	data, err := ToJSON(s)
	if err != nil {
		return "", fmt.Errorf("formatting stack JSON: %w", err)
	}
	tmp := target + fmt.Sprintf(".tmp-%d", os.Getpid())
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return "", fmt.Errorf("writing temporary stack file: %w", err)
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("persisting stack file: %w", err)
	}
	s.Source = target
	return target, nil
}
