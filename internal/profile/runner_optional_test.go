package profile

import (
	"os/exec"
	"testing"
)

func TestCommandNotFound(t *testing.T) {
	if !commandNotFound(exec.Command("sh", "-c", "exit 127").Run()) {
		t.Error("exit 127 must count as command not found")
	}
	if commandNotFound(exec.Command("sh", "-c", "exit 1").Run()) {
		t.Error("exit 1 is a real failure")
	}
}

func TestOptionalTestSkippedWhenCommandMissing(t *testing.T) {
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bwrap not installed")
	}
	args := []string{"--ro-bind", "/", "/", "--dev", "/dev", "--proc", "/proc"}
	spec := TestSpec{Name: "missing", Cmd: []string{"sh", "-c", "bws_no_such_binary --version"}, Optional: true}
	if got := runProfileTest(spec, nil, args); got.Status != "skipped" {
		t.Fatalf("optional test status = %q (%s), want skipped", got.Status, got.Output)
	}
	spec.Optional = false
	if got := runProfileTest(spec, nil, args); got.Status != "failed" {
		t.Fatalf("required test status = %q, want failed", got.Status)
	}
}
