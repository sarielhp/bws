package stack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEmbeddedStacksLoad(t *testing.T) {
	seeds := []string{"go-agent", "latex-review", "python-uv", "rust-dev"}
	for _, name := range seeds {
		stk, err := Get(name)
		if err != nil {
			t.Fatalf("failed to get embedded stack %q: %v", name, err)
		}
		if stk.Name != name {
			t.Errorf("expected stack name %q, got %q", name, stk.Name)
		}
		if len(stk.Profiles) == 0 {
			t.Errorf("stack %q has no profiles", name)
		}
		digest, err := Digest(stk)
		if err != nil || len(digest) != 64 {
			t.Errorf("invalid digest for stack %q: %s, err: %v", name, digest, err)
		}
		if stk.Source != "embedded" {
			t.Errorf("expected source 'embedded', got %q", stk.Source)
		}
	}
}

func TestStackValidation(t *testing.T) {
	tests := []struct {
		name    string
		stack   *Stack
		wantErr bool
	}{
		{
			name:    "nil stack",
			stack:   nil,
			wantErr: true,
		},
		{
			name: "empty name",
			stack: &Stack{
				Name:     "",
				Profiles: []string{"go"},
			},
			wantErr: true,
		},
		{
			name: "invalid name with path traversal",
			stack: &Stack{
				Name:     "../escape",
				Profiles: []string{"go"},
			},
			wantErr: true,
		},
		{
			name: "empty profiles",
			stack: &Stack{
				Name:     "empty-prof",
				Profiles: []string{},
			},
			wantErr: true,
		},
		{
			name: "invalid profile reference",
			stack: &Stack{
				Name:     "bad-prof",
				Profiles: []string{"../bad"},
			},
			wantErr: true,
		},
		{
			name: "valid stack",
			stack: &Stack{
				Name:     "valid-stack",
				Category: "Custom",
				Profiles: []string{"go", "git"},
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.stack)
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

func TestStackDigestDeterminism(t *testing.T) {
	stk1 := &Stack{
		Name:        "custom-persona",
		Title:       "Custom Persona",
		Description: "A custom dev environment",
		Category:    "Testing",
		Profiles:    []string{"editor", "git"},
		Source:      "/tmp/old/path.json",
		Provenance: &Provenance{
			SourceWorkspace: "/tmp/old/workspace",
			SavedAt:         time.Now().Add(-1 * time.Hour),
			VerifiedDigest:  "old-digest",
		},
	}

	stk2 := &Stack{
		Name:        "custom-persona",
		Title:       "Custom Persona",
		Description: "A custom dev environment",
		Category:    "Testing",
		Profiles:    []string{"editor", "git"},
		Source:      "/tmp/new/path.json",
		Provenance: &Provenance{
			SourceWorkspace: "/tmp/new/workspace",
			SavedAt:         time.Now(),
			VerifiedDigest:  "new-digest",
		},
	}

	d1, err := Digest(stk1)
	if err != nil {
		t.Fatalf("Digest stk1 failed: %v", err)
	}
	d2, err := Digest(stk2)
	if err != nil {
		t.Fatalf("Digest stk2 failed: %v", err)
	}

	if d1 != d2 {
		t.Errorf("expected deterministic digest ignoring source/provenance, got %s != %s", d1, d2)
	}
}

func TestStackParseJSONC(t *testing.T) {
	jsonc := []byte(`{
		// Trailing comments and comma test
		"name": "jsonc-stack",
		"title": "JSONC Stack",
		"category": "Testing",
		"profiles": [
			"go",
			"git",
		],
	}`)

	stk, err := Parse(jsonc)
	if err != nil {
		t.Fatalf("failed to parse JSONC stack: %v", err)
	}
	if stk.Name != "jsonc-stack" {
		t.Errorf("expected name 'jsonc-stack', got %q", stk.Name)
	}
	if len(stk.Profiles) != 2 {
		t.Errorf("expected 2 profiles, got %d", len(stk.Profiles))
	}
}

func TestUserStackSaveAndGet(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	stk := &Stack{
		Name:        "my-test-stack",
		Title:       "My Test Stack",
		Description: "Saved from unit test",
		Category:    "User Saved",
		Profiles:    []string{"editor", "no-history"},
		Env: map[string]string{
			"TEST_VAR": "true",
		},
		Provenance: &Provenance{
			SourceWorkspace: "/workspaces/my-test",
			SavedAt:         time.Now().UTC(),
		},
	}

	path, err := SaveUserStack(stk)
	if err != nil {
		t.Fatalf("SaveUserStack failed: %v", err)
	}
	expectedPath := filepath.Join(tempHome, ".config", "bws", "stacks", "my-test-stack.json")
	if path != expectedPath {
		t.Errorf("expected path %s, got %s", expectedPath, path)
	}

	loaded, err := Get("my-test-stack")
	if err != nil {
		t.Fatalf("Get saved stack failed: %v", err)
	}
	if loaded.Title != stk.Title {
		t.Errorf("expected title %q, got %q", stk.Title, loaded.Title)
	}

	list, err := List("User Saved")
	if err != nil {
		t.Fatalf("List User Saved failed: %v", err)
	}
	found := false
	for _, s := range list {
		if s.Name == "my-test-stack" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected to find 'my-test-stack' in User Saved list")
	}

	searchRes, err := Search("unit test")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(searchRes) == 0 || searchRes[0].Name != "my-test-stack" {
		t.Errorf("Search by description failed to find stack: %+v", searchRes)
	}
}

func TestSaveUserStackAtomicClean(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	stk := &Stack{
		Name:        "atomic-test",
		Title:       "Atomic Persistence Test",
		Description: "Testing atomic tempfile creation and fsync",
		Profiles:    []string{"go"},
	}

	path, err := SaveUserStack(stk)
	if err != nil {
		t.Fatalf("SaveUserStack failed: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected target stack file to exist: %v", err)
	}

	dir := UserStacksDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading user stacks dir: %v", err)
	}

	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") {
			t.Errorf("found leftover temporary file in user stacks dir: %s", entry.Name())
		}
	}
}
