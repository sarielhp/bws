package cli

import (
	"testing"

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
