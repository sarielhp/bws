package config

import (
	"os"
	"path/filepath"
	"testing"
)

func mkws(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".bws"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".bws", "config.jsonc"), []byte("{}\n"), 0644); err != nil {
		t.Fatal(err)
	}
}

// The Target of the chain must always equal what FindWorkspaceRoot resolves,
// so the banner and reads can never disagree.
func TestWorkspaceCandidatesTargetMatchesResolve(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	root := filepath.Join(home, "proj")
	sub := filepath.Join(root, "sub")
	deep := filepath.Join(sub, "deep")
	if err := os.MkdirAll(deep, 0755); err != nil {
		t.Fatal(err)
	}
	mkws(t, root)

	_, wantCfg := FindWorkspaceRoot(deep)
	chain := WorkspaceCandidates(deep)

	var target *WorkspaceCandidate
	for i := range chain {
		if chain[i].Target {
			target = &chain[i]
			break
		}
	}
	if target == nil {
		t.Fatal("no target in chain")
	}
	if target.ConfigPath != wantCfg {
		t.Errorf("target %q != resolved %q", target.ConfigPath, wantCfg)
	}
	if target.Dir != root {
		t.Errorf("target dir = %q, want %q", target.Dir, root)
	}
}

func TestWorkspaceCandidatesShowsAncestorAndCwd(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	root := filepath.Join(home, "proj")
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	mkws(t, root)
	mkws(t, sub)

	chain := WorkspaceCandidates(sub)
	if len(chain) < 2 {
		t.Fatalf("expected cwd + ancestor candidates, got %d", len(chain))
	}
	if !chain[0].Exists || chain[0].Dir != sub || !chain[0].Target {
		t.Errorf("nearest (cwd) should exist and be the target: %+v", chain[0])
	}
	if !chain[1].Exists || chain[1].Dir != root || chain[1].Target {
		t.Errorf("ancestor should exist and be shadowed: %+v", chain[1])
	}
}

func TestWorkspaceCandidatesFreshCwdIsTarget(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	root := filepath.Join(home, "proj")
	sub := filepath.Join(root, "fresh")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	mkws(t, root) // ancestor exists, but cwd has none

	chain := WorkspaceCandidates(sub)
	// With no config in cwd, the nearest existing config (the ancestor) is the
	// target, matching reads. cwd is listed as a non-existing candidate.
	if chain[0].Dir != sub || chain[0].Target || chain[0].Exists {
		t.Errorf("cwd should be a non-existing, non-target candidate: %+v", chain[0])
	}
	if chain[1].Dir != root || !chain[1].Exists || !chain[1].Target {
		t.Errorf("ancestor should exist and be the write target: %+v", chain[1])
	}
}

func TestWorkspaceCandidatesNoConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, "bare")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	chain := WorkspaceCandidates(dir)
	if len(chain) != 1 || !chain[0].Target || chain[0].Exists {
		t.Errorf("unexpected chain: %+v", chain)
	}
}
