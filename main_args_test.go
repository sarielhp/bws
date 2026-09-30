package main

import (
	"reflect"
	"testing"
)

func TestNormalizeArgs_Help(t *testing.T) {
	tests := []struct {
		input    []string
		expected []string
	}{
		{input: []string{"help"}, expected: []string{"--help"}},
		{input: []string{"-help"}, expected: []string{"--help"}},
		{input: []string{"--h"}, expected: []string{"--help"}},
		{input: []string{"-?"}, expected: []string{"--help"}},
		{input: []string{"-H"}, expected: []string{"--help"}},
		{input: []string{"status", "help"}, expected: []string{"status", "help"}},
	}

	for _, tc := range tests {
		got := normalizeArgs(tc.input)
		if !reflect.DeepEqual(got, tc.expected) {
			t.Errorf("normalizeArgs(%v) = %v, want %v", tc.input, got, tc.expected)
		}
	}
}

func TestNormalizeArgs_HoistSubcommand(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "flags before plan with argument",
			input:    []string{"--max-file-count", "-1", "plan"},
			expected: []string{"plan", "--max-file-count", "-1"},
		},
		{
			name:     "verbose before status",
			input:    []string{"-v", "status"},
			expected: []string{"status", "-v"},
		},
		{
			name:     "force before init with subargs",
			input:    []string{"-f", "init", "-s", "latex-review"},
			expected: []string{"init", "-f", "-s", "latex-review"},
		},
		{
			name:     "flags before compound config set with negative value",
			input:    []string{"-g", "config", "set", "max_file_count", "-1"},
			expected: []string{"config", "set", "-g", "max_file_count", "--", "-1"},
		},
		{
			name:     "non-subcommand not hoisted",
			input:    []string{"-v", "ls", "-la"},
			expected: []string{"-v", "ls", "-la"},
		},
		{
			name:     "stop at dash-dash",
			input:    []string{"-v", "--", "status"},
			expected: []string{"-v", "--", "status"},
		},
		{
			name:     "already command-first",
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
