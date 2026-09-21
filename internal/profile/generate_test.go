package profile

import (
	"reflect"
	"testing"
)

func TestFilterValidDependencies(t *testing.T) {
	registry := map[string]*Profile{
		"python": {
			Name: "python",
		},
		"git": {
			Name: "git",
		},
		"agy": {
			Name:    "agy",
			Aliases: []string{"antigravity"},
		},
		"antigravity": {
			Name: "agy",
		},
		"fish": {
			Name: "fish",
		},
	}

	tests := []struct {
		name     string
		deps     []string
		selfName string
		expected []string
	}{
		{
			name:     "filters unmapped C libraries",
			deps:     []string{"pcre2", "ncurses", "glibc"},
			selfName: "fish",
			expected: nil,
		},
		{
			name:     "maps exact profile names",
			deps:     []string{"git"},
			selfName: "tool",
			expected: []string{"git"},
		},
		{
			name:     "maps version-suffixed dependencies to canonical name",
			deps:     []string{"python@3.12", "openssl@3"},
			selfName: "tool",
			expected: []string{"python"},
		},
		{
			name:     "maps aliases to canonical profile name",
			deps:     []string{"antigravity"},
			selfName: "tool",
			expected: []string{"agy"},
		},
		{
			name:     "filters self-dependency and duplicates",
			deps:     []string{"fish", "git", "git", "pcre2"},
			selfName: "fish",
			expected: []string{"git"},
		},
		{
			name:     "empty dependencies",
			deps:     []string{},
			selfName: "tool",
			expected: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := filterValidDependencies(tc.deps, tc.selfName, registry)
			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("filterValidDependencies(%v, %q) = %v; want %v", tc.deps, tc.selfName, got, tc.expected)
			}
		})
	}
}

func TestGenerateProfileEmptyName(t *testing.T) {
	_, err := GenerateProfile("", nil)
	if err == nil {
		t.Errorf("expected error for empty profile name, got nil")
	}
}

func TestGenerateProfile(t *testing.T) {
	registry := map[string]*Profile{
		"git": {Name: "git"},
	}

	p, err := GenerateProfile("fish", registry)
	if err != nil {
		t.Fatalf("GenerateProfile(fish) failed: %v", err)
	}

	if p.Name != "fish" {
		t.Errorf("expected name fish, got %s", p.Name)
	}
	if len(p.Requires) > 0 {
		t.Errorf("expected no unresolved requires (like pcre2) for fish, got: %v", p.Requires)
	}
	if len(p.Tests) == 0 {
		t.Errorf("expected tests to be generated, got none")
	}
	if p.Detect == nil {
		t.Errorf("expected detect spec to be generated, got nil")
	}
}

func TestIsToolProfile(t *testing.T) {
	tests := []struct {
		profile  *Profile
		expected bool
	}{
		{profile: &Profile{Name: "git"}, expected: true},
		{profile: &Profile{Name: "rust"}, expected: true},
		{profile: &Profile{Name: "no-net"}, expected: false},
		{profile: &Profile{Name: "no-sudo"}, expected: false},
		{profile: &Profile{Name: "mask-sudo"}, expected: false},
		{profile: &Profile{Name: "offline"}, expected: false},
		{profile: &Profile{Name: "python-dev", Kind: "compound"}, expected: false},
		{profile: nil, expected: false},
	}
	for _, tc := range tests {
		got := IsToolProfile(tc.profile)
		if got != tc.expected {
			var name string
			if tc.profile != nil {
				name = tc.profile.Name
			}
			t.Errorf("IsToolProfile(%q) = %v; want %v", name, got, tc.expected)
		}
	}
}

func TestVerifyToolInstalled(t *testing.T) {
	p := &Profile{
		Name: "git",
		Tests: []TestSpec{
			{Cmd: []string{"git", "--version"}, Type: "version"},
		},
	}
	path, err := VerifyToolInstalled("git", p)
	if err != nil || path == "" {
		t.Errorf("expected git to be verified as installed, got path=%q, err=%v", path, err)
	}

	pFake := &Profile{
		Name: "mongishogi_nonexistent",
		Tests: []TestSpec{
			{Cmd: []string{"mongishogi_nonexistent", "--version"}, Type: "version"},
		},
	}
	_, err = VerifyToolInstalled("mongishogi_nonexistent", pFake)
	if err == nil {
		t.Errorf("expected error for non-existent tool, got nil")
	}
}
