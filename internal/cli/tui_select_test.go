package cli

import (
	"bytes"
	"strings"
	"testing"

	"bws/internal/config"
	"bws/internal/stack"
)

func TestReadSelectorKey(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantAct  keyAction
		wantJump int
	}{
		{"arrow up", "\x1b[A", actUp, 0},
		{"arrow down", "\x1b[B", actDown, 0},
		{"vim up", "k", actUp, 0},
		{"vim down", "j", actDown, 0},
		{"enter newline", "\n", actSelect, 0},
		{"enter return", "\r", actSelect, 0},
		{"esc cancel", "\x1b", actCancel, 0},
		{"q cancel", "q", actCancel, 0},
		{"ctrl-c cancel", "\x03", actCancel, 0},
		{"jump 1", "1", actJump, 0},
		{"jump 4", "4", actJump, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			act, jump, err := readSelectorKey(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if act != tt.wantAct {
				t.Errorf("got action %v, want %v", act, tt.wantAct)
			}
			if jump != tt.wantJump {
				t.Errorf("got jump %d, want %d", jump, tt.wantJump)
			}
		})
	}
}

func TestRenderStackSelector(t *testing.T) {
	stk := &stack.Stack{
		Name:        "test-agent",
		Title:       "Test Agent Persona",
		Category:    "AI & Autonomous",
		Description: "A test sandbox persona",
		Profiles:    []string{"go", "no-sudo", "no-forge"},
	}

	items := []StackChoice{
		{Stack: stk, Badge: "Recommended"},
		{Label: "Basic (raw)", IsBasic: true, Badge: "Profiles"},
	}

	var buf bytes.Buffer
	linesCount := renderStackSelector(&buf, "Select Stack", items, 0, 80, 24)
	if linesCount <= 0 {
		t.Errorf("expected positive line count, got %d", linesCount)
	}

	out := buf.String()
	if !strings.Contains(out, "test-agent") {
		t.Errorf("expected output to contain 'test-agent', got:\n%s", out)
	}
	if !strings.Contains(out, "Stack Details") {
		t.Errorf("expected output to contain 'Stack Details', got:\n%s", out)
	}
	if !strings.Contains(out, "No-Sudo") {
		t.Errorf("expected output to contain '[No-Sudo]', got:\n%s", out)
	}
	if !strings.Contains(out, "No-Forge") {
		t.Errorf("expected output to contain '[No-Forge]', got:\n%s", out)
	}
}

func TestRenderStackSelectorNarrowTerminal(t *testing.T) {
	stk := &stack.Stack{
		Name:        "latex-review",
		Title:       "LaTeX Manuscript Review",
		Category:    "Document & Publishing",
		Description: "LaTeX authoring, inspection, and compilation sandbox for manuscript reviews",
		Profiles:    []string{"latex-dev", "no-history", "no-secrets"},
	}

	items := []StackChoice{
		{Stack: stk, Badge: "Matches *.tex, latexmkrc"},
		{Label: "Basic (detected tool profiles)", IsBasic: true, Badge: "Profiles"},
	}

	widths := []int{100, 80, 60, 45, 35, 25}
	heights := []int{24, 12, 8}

	for _, w := range widths {
		for _, h := range heights {
			var buf bytes.Buffer
			linesCount := renderStackSelector(&buf, "Recommended Stacks for this workspace", items, 0, w, h)

			if linesCount > h {
				t.Errorf("w=%d h=%d: rendered %d lines, exceeded termHeight %d", w, h, linesCount, h)
			}

			rawLines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
			if len(rawLines) != linesCount {
				t.Errorf("w=%d h=%d: split count %d != linesCount %d", w, h, len(rawLines), linesCount)
			}

			for lineIdx, line := range rawLines {
				clean := stripANSI(line)
				clean = strings.TrimPrefix(clean, "\r\x1b[2K")
				vw := visualWidth(clean)
				if vw > w {
					t.Errorf("w=%d h=%d line %d: visual width %d exceeded terminal width %d:\n%q", w, h, lineIdx, vw, w, clean)
				}
			}
		}
	}
}

func TestExtractSecurityHighlights(t *testing.T) {
	tTrue := true
	stk := &stack.Stack{
		Profiles: []string{"go", "no-sudo", "no-ssh", "no-forge"},
		Features: &config.FeaturesConfig{
			NoNet: &tTrue,
		},
	}

	tags := extractSecurityHighlights(stk)
	joined := strings.Join(tags, " ")
	for _, expected := range []string{"No-Sudo", "No-SSH", "No-Forge", "Offline"} {
		if !strings.Contains(joined, expected) {
			t.Errorf("expected security tags to contain %s, got: %s", expected, joined)
		}
	}
}
