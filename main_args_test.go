package main

import (
	"reflect"
	"testing"

	"github.com/sarielhp/clihelp/clihelptest"
)

// TestNativeHelpRouting verifies that clihelp itself, without bws rewriting the
// arguments, resolves the help flags and the help topics.
func TestNativeHelpRouting(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantSub string
	}{
		{"bare help lists commands", []string{"help"}, "Commands:"},
		{"help flags shows grouped flags page", []string{"help", "flags"}, "Global flags available to all commands:"},
		{"help topics lists topics", []string{"help", "topics"}, "Help Topics:"},
		{"-H extended help includes the global note", []string{"-H"}, "Bws runs isolated"},
		{"--help extended help", []string{"--help"}, "Commands:"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := clihelptest.Execute(buildApp(), tc.args)
			res.AssertNoError(t)
			res.AssertStdoutContains(t, tc.wantSub)
		})
	}
}

// TestNormalizeArgs_Passthrough checks the rewrites bws still needs: the learn
// subcommand's "--" separator, and nothing else.
func TestNormalizeArgs_Passthrough(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "learn inserts dash-dash before the traced command",
			input:    []string{"learn", "bash"},
			expected: []string{"learn", "--", "bash"},
		},
		{
			name:     "learn keeps an existing dash-dash",
			input:    []string{"learn", "--", "bash"},
			expected: []string{"learn", "--", "bash"},
		},
		{
			name:     "run passes program flags to the program",
			input:    []string{"run", "git", "--version"},
			expected: []string{"run", "--", "git", "--version"},
		},
		{
			name:     "exec passes shorthand flags to the program",
			input:    []string{"exec", "sh", "-c", "true"},
			expected: []string{"exec", "--", "sh", "-c", "true"},
		},
		{
			name:     "bws flags before the program stay with bws",
			input:    []string{"-N", "run", "-v", "--max-file-count", "5", "tmux", "-V"},
			expected: []string{"-N", "run", "-v", "--max-file-count", "5", "--", "tmux", "-V"},
		},
		{
			name:     "run keeps an existing dash-dash",
			input:    []string{"run", "--", "git", "--version"},
			expected: []string{"run", "--", "git", "--version"},
		},
		{
			name:     "run without a program is left for clihelp to report",
			input:    []string{"run", "-h"},
			expected: []string{"run", "-h"},
		},
		{
			name:     "learn skips its --profile value",
			input:    []string{"learn", "-p", "mytool", "mytool", "--help"},
			expected: []string{"learn", "-p", "mytool", "--", "mytool", "--help"},
		},
		{
			name:     "the word learn inside run arguments is not a command",
			input:    []string{"run", "grep", "learn", "-r", "."},
			expected: []string{"run", "--", "grep", "learn", "-r", "."},
		},
		{
			name:     "a run argument to another command is untouched",
			input:    []string{"status", "run", "-v"},
			expected: []string{"status", "run", "-v"},
		},
		{
			name:     "non-learn args pass through unchanged",
			input:    []string{"status", "-v"},
			expected: []string{"status", "-v"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeArgs(tc.input)
			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("normalizeArgs(%v) = %v, want %v", tc.input, got, tc.expected)
			}
		})
	}
}

func TestNormalizeConfigSet(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "negative integer value",
			input:    []string{"config", "set", "max_file_count", "-1"},
			expected: []string{"config", "set", "max_file_count", "--", "-1"},
		},
		{
			name:     "conf alias with negative value",
			input:    []string{"conf", "set", "max_file_count", "-1"},
			expected: []string{"conf", "set", "max_file_count", "--", "-1"},
		},
		{
			name:     "global flag before key",
			input:    []string{"config", "set", "-g", "max_file_count", "-1"},
			expected: []string{"config", "set", "-g", "max_file_count", "--", "-1"},
		},
		{
			name:     "global flag after value",
			input:    []string{"config", "set", "max_file_count", "-1", "-g"},
			expected: []string{"config", "set", "-g", "max_file_count", "--", "-1"},
		},
		{
			name:     "positive boolean value left untouched",
			input:    []string{"config", "set", "no_file_limit", "true"},
			expected: []string{"config", "set", "no_file_limit", "true"},
		},
		{
			name:     "explicit dash-dash left untouched",
			input:    []string{"config", "set", "max_file_count", "--", "-1"},
			expected: []string{"config", "set", "max_file_count", "--", "-1"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeConfigSet(tc.input)
			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("normalizeConfigSet(%v) = %v, want %v", tc.input, got, tc.expected)
			}
		})
	}
}
