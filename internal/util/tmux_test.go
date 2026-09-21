package util

import (
	"testing"
)

func TestIsInsideTmux(t *testing.T) {
	t.Setenv("TMUX", "")
	if IsInsideTmux() {
		t.Errorf("expected IsInsideTmux() = false when TMUX is empty")
	}

	t.Setenv("TMUX", "/tmp/tmux-1000/default,123,0")
	if !IsInsideTmux() {
		t.Errorf("expected IsInsideTmux() = true when TMUX is set")
	}
}

func TestSetHostTmuxTitleWhenNotInsideTmux(t *testing.T) {
	t.Setenv("TMUX", "")
	cleanup, err := SetHostTmuxTitle()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if cleanup == nil {
		t.Fatalf("expected non-nil cleanup function")
	}
	cleanup()
}
