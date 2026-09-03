package config

import (
	"encoding/json"
	"testing"
)

func TestFeaturesConfigUnmarshalAutoInit(t *testing.T) {
	tests := []struct {
		name     string
		jsonStr  string
		expected string
		wantErr  bool
	}{
		{
			name:     "string always",
			jsonStr:  `{"auto_init": "always"}`,
			expected: "always",
		},
		{
			name:     "string prompt",
			jsonStr:  `{"auto_init": "prompt"}`,
			expected: "prompt",
		},
		{
			name:     "string never",
			jsonStr:  `{"auto_init": "never"}`,
			expected: "never",
		},
		{
			name:     "bool true to always",
			jsonStr:  `{"auto_init": true}`,
			expected: "always",
		},
		{
			name:     "bool false to never",
			jsonStr:  `{"auto_init": false}`,
			expected: "never",
		},
		{
			name:     "unset stays empty",
			jsonStr:  `{}`,
			expected: "",
		},
		{
			name:     "null stays empty",
			jsonStr:  `{"auto_init": null}`,
			expected: "",
		},
		{
			name:    "invalid number",
			jsonStr: `{"auto_init": 123}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var f FeaturesConfig
			err := json.Unmarshal([]byte(tt.jsonStr), &f)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %s, got nil", tt.jsonStr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %s: %v", tt.jsonStr, err)
			}
			if f.AutoInit != tt.expected {
				t.Errorf("expected AutoInit %q, got %q", tt.expected, f.AutoInit)
			}
		})
	}
}

func TestAutoInitModeDefaultsAndResolution(t *testing.T) {
	if mode := AutoInitMode(nil); mode != "always" {
		t.Errorf("expected default 'always' for nil config, got %q", mode)
	}

	cfg := &Config{}
	if mode := AutoInitMode(cfg); mode != "always" {
		t.Errorf("expected default 'always' for empty config, got %q", mode)
	}

	modes := map[string]string{
		"":       "always",
		"always": "always",
		"on":     "always",
		"true":   "always",
		"prompt": "prompt",
		"ask":    "prompt",
		"never":  "never",
		"off":    "never",
		"false":  "never",
		"custom": "custom",
	}

	for input, expected := range modes {
		c := &Config{Features: &FeaturesConfig{AutoInit: input}}
		if got := AutoInitMode(c); got != expected {
			t.Errorf("AutoInitMode(%q) = %q; want %q", input, got, expected)
		}
	}
}

func TestMergeFeaturesAutoInit(t *testing.T) {
	global := &FeaturesConfig{AutoInit: "never"}
	local := &FeaturesConfig{}
	merged := MergeFeatures(global, local)
	if merged.AutoInit != "never" {
		t.Errorf("expected global AutoInit 'never' preserved, got %q", merged.AutoInit)
	}

	local.AutoInit = "always"
	merged = MergeFeatures(global, local)
	if merged.AutoInit != "always" {
		t.Errorf("expected local AutoInit 'always' to override, got %q", merged.AutoInit)
	}
}

func TestProjectFeaturesDetectionHelpers(t *testing.T) {
	pf := ProjectFeatures{}
	if pf.AnyDetected() {
		t.Error("expected AnyDetected() false for empty features")
	}
	if len(pf.DetectedStacks()) != 0 {
		t.Errorf("expected empty stacks, got %v", pf.DetectedStacks())
	}

	pf.HasGo = true
	if !pf.AnyDetected() {
		t.Error("expected AnyDetected() true when HasGo is true")
	}
	stacks := pf.DetectedStacks()
	if len(stacks) != 1 || stacks[0] != "Go" {
		t.Errorf("expected ['Go'], got %v", stacks)
	}

	pf.HasPython = true
	pf.HasRust = true
	pf.HasNode = true
	pf.HasLatex = true
	pf.HasOpenCode = true

	allStacks := pf.DetectedStacks()
	expected := []string{"Go", "Python/UV", "Rust", "Node", "LaTeX/TeX", "OpenCode"}
	if len(allStacks) != len(expected) {
		t.Fatalf("expected %d stacks, got %d: %v", len(expected), len(allStacks), allStacks)
	}
	for i, name := range expected {
		if allStacks[i] != name {
			t.Errorf("stack[%d]: expected %q, got %q", i, name, allStacks[i])
		}
	}
}
