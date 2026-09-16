package cli

import (
	"bytes"
	"strings"
	"testing"

	"bws/internal/config"
	"bws/internal/policy"
)

func TestConsecutiveInteractiveAnswers(t *testing.T) {
	input := strings.NewReader("1\ny\n")
	choice, err := readPolicyAnswer(input)
	if err != nil || choice != "1" {
		t.Fatalf("%q %v", choice, err)
	}
	if err := confirmPolicy(input, &bytes.Buffer{}, "Confirm?"); err != nil {
		t.Fatal(err)
	}
	if err := confirmPolicy(strings.NewReader(""), &bytes.Buffer{}, "Confirm?"); err == nil {
		t.Fatal("EOF approved a write")
	}
}

func TestExportAcknowledgments(t *testing.T) {
	plan := &policy.ExportPlan{Unsupported: []string{"sandbox_path"}, MachinePaths: []string{"/opt/compiler"}}
	if err := acknowledgeExport(plan, ProfileSaveOptions{Force: true, Yes: true}); err == nil {
		t.Fatal("force/yes bypassed limitations")
	}
	opts := ProfileSaveOptions{Omit: []string{"sandbox_path"}, AllowMachinePaths: true}
	if err := acknowledgeExport(plan, opts); err != nil {
		t.Fatal(err)
	}
	opts.Omit = append(opts.Omit, "misspelled")
	if err := acknowledgeExport(plan, opts); err == nil {
		t.Fatal("unknown omission accepted")
	}
}

func TestPermissionSummaryRedactsEnvironment(t *testing.T) {
	var output bytes.Buffer
	cfg := &config.Config{Env: map[string]string{"TOKEN": "not-for-display"}}
	PrintPolicySummary(&output, cfg)
	if strings.Contains(output.String(), "not-for-display") || !strings.Contains(output.String(), "TOKEN") {
		t.Fatal(output.String())
	}
}
