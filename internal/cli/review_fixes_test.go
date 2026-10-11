package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

func TestPatchReviewedStackPreservesCommentsAndBinds(t *testing.T) {
	input := []byte(`{
  // Stack configuration
  "stack": "go-agent",
  // Build cache bind
  "binds_rw": [
    "~/.cache/go-build",
    ["/host/bin", "/guest/bin"]
  ],
  "home": "@@HOME@@/workspace"
}`)

	approval := &config.ProfileApproval{
		Source: "embedded",
		SHA256: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
	}

	patched, err := patchReviewedStack(input, approval)
	if err != nil {
		t.Fatalf("patchReviewedStack failed: %v", err)
	}

	patchedStr := string(patched)
	t.Logf("first patched:\n%s", patchedStr)
	if !strings.Contains(patchedStr, "// Stack configuration") {
		t.Errorf("comments lost: %s", patchedStr)
	}
	if !strings.Contains(patchedStr, "// Build cache bind") {
		t.Errorf("bind comments lost: %s", patchedStr)
	}
	if !strings.Contains(patchedStr, `"~/.cache/go-build"`) {
		t.Errorf("bind string corrupted: %s", patchedStr)
	}
	if !strings.Contains(patchedStr, `"@@HOME@@/workspace"`) {
		t.Errorf("@@HOME@@ token lost: %s", patchedStr)
	}
	if !strings.Contains(patchedStr, `"reviewed_stack"`) || !strings.Contains(patchedStr, "abcdef1234567890") {
		t.Errorf("reviewed_stack not properly set: %s", patchedStr)
	}

	parsed, err := config.Parse(bytes.Clone(patched), "/test/path/config.jsonc")
	if err != nil {
		t.Fatalf("parsed patched config failed: %v", err)
	}
	if parsed.ReviewedStack == nil || parsed.ReviewedStack.SHA256 != approval.SHA256 {
		t.Errorf("parsed reviewed stack mismatch: %+v", parsed.ReviewedStack)
	}
	if len(parsed.BindsRW) != 2 {
		t.Fatalf("expected 2 binds_rw, got %d", len(parsed.BindsRW))
	}

	// Now update an already existing reviewed_stack
	secondApproval := &config.ProfileApproval{
		Source: "upstream",
		SHA256: "9999999999999999999999999999999999999999999999999999999999999999",
	}
	patched2, err := patchReviewedStack(patched, secondApproval)
	if err != nil {
		t.Fatalf("patchReviewedStack second update failed: %v", err)
	}
	parsed2, err := config.Parse(bytes.Clone(patched2), "/test/path/config.jsonc")
	if err != nil {
		t.Fatalf("parsed second patched config failed: %v", err)
	}
	if parsed2.ReviewedStack == nil || parsed2.ReviewedStack.SHA256 != secondApproval.SHA256 {
		t.Errorf("second update failed to overwrite: %+v", parsed2.ReviewedStack)
	}
	if !strings.Contains(string(patched2), "// Stack configuration") {
		t.Errorf("comments lost in second update: %s", string(patched2))
	}
}

func TestProfileUpdateAndNewRefuseOverwriteWithoutForce(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	cwd := t.TempDir()
	t.Chdir(cwd)

	globalDir := filepath.Join(tmpHome, ".config", "bws", "profiles")
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatalf("mkdir profiles: %v", err)
	}

	profilePath := filepath.Join(globalDir, "mycustom.json")
	customContent := []byte(`{"name":"mycustom","description":"Custom user profile"}`)
	if err := config.WriteTrustedFile(profilePath, customContent); err != nil {
		t.Fatalf("write profile: %v", err)
	}

	// 1. HandleProfileNew must fail if destination exists and !force
	if err := HandleProfileNew("mycustom", true, false, false); err == nil {
		t.Error("HandleProfileNew must refuse to overwrite existing profile when force=false")
	}

	// 2. HandleProfileFetch must fail if destination exists and !force
	if err := HandleProfileFetch("mycustom", true, false, false); err == nil {
		t.Error("HandleProfileFetch must refuse to overwrite existing profile when force=false")
	}

	// 3. HandleProfileUpdate must not synthesize or clobber local-only profile
	if err := HandleProfileUpdate(); err != nil {
		t.Fatalf("HandleProfileUpdate failed: %v", err)
	}

	// Verify original custom content remains intact
	after, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("read after update: %v", err)
	}
	if string(after) != string(customContent) {
		t.Errorf("HandleProfileUpdate clobbered custom profile: got %s, want %s", string(after), string(customContent))
	}
}
