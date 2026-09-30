package cli

import (
	"slices"
	"testing"

	"bws/internal/config"
	"bws/internal/learn"
)

func TestProfileWritersRejectTraversalNames(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())
	bad := "../../../../tmp/escape"

	if err := HandleProfileNew(bad, true, false, true); err == nil {
		t.Error("profile generate accepted a traversal name")
	}
	if err := HandleProfileFetch(bad, true, false, true); err == nil {
		t.Error("profile fetch accepted a traversal name")
	}
	if err := handleProfileGeneration(&learn.TraceResult{}, bad, true, true, false); err == nil {
		t.Error("learn -p accepted a traversal name")
	}
	if err := ensureProfileInRegistry(bad, false, true, true, true, t.TempDir(), nil); err == nil {
		t.Error("add -c accepted a traversal name")
	}
}

func TestMatchBinaryHostAmbiguousBasename(t *testing.T) {
	t.Setenv("HOME", "/home/u")
	binds := []config.BindEntry{{Host: "./aa"}, {Host: "/home/u/bin/aa"}}
	if _, err := matchBinaryHost(binds, "aa"); err == nil {
		t.Fatal("ambiguous basename must be rejected")
	}
	got, err := matchBinaryHost(binds, "/home/u/bin/aa")
	if err != nil || got != "/home/u/bin/aa" {
		t.Fatalf("exact path: got %q, %v", got, err)
	}
}

func TestEditorCommandSplitsArgs(t *testing.T) {
	cmd := editorCommand("emacs -nw", "/tmp/c.jsonc")
	want := []string{"emacs", "-nw", "/tmp/c.jsonc"}
	if !slices.Equal(cmd.Args, want) {
		t.Fatalf("args = %v, want %v", cmd.Args, want)
	}
}
