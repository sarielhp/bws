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

func TestParseTmuxOutputWithColons(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantNil   bool
		wantPane  string
		wantWin   string
		wantAuto  bool
		wantTitle string
	}{
		{
			name:      "window with colons",
			input:     "%1\tdev:server.go:8080\t1\tbash",
			wantPane:  "%1",
			wantWin:   "dev:server.go:8080",
			wantAuto:  true,
			wantTitle: "bash",
		},
		{
			name:      "pane title with colons and spaces",
			input:     "%2\tmain\t0\tnode /app/index.js:3000",
			wantPane:  "%2",
			wantWin:   "main",
			wantAuto:  false,
			wantTitle: "node /app/index.js:3000",
		},
		{
			name:    "empty string",
			input:   "",
			wantNil: true,
		},
		{
			name:    "missing fields",
			input:   "%1\tmain\t1",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTmuxOutput(tt.input)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("expected nil state, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected non-nil state")
			}
			if got.PaneID != tt.wantPane {
				t.Errorf("got PaneID %q, want %q", got.PaneID, tt.wantPane)
			}
			if got.WindowName != tt.wantWin {
				t.Errorf("got WindowName %q, want %q", got.WindowName, tt.wantWin)
			}
			if got.AutoRename != tt.wantAuto {
				t.Errorf("got AutoRename %v, want %v", got.AutoRename, tt.wantAuto)
			}
			if got.PaneTitle != tt.wantTitle {
				t.Errorf("got PaneTitle %q, want %q", got.PaneTitle, tt.wantTitle)
			}
		})
	}
}
