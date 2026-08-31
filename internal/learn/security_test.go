package learn

import (
	"testing"
)

func TestSecurityDenylist(t *testing.T) {
	homeDir := "/home/testuser"

	tests := []struct {
		path      string
		sensitive bool
	}{
		{"/", true},
		{"/etc", true},
		{"/etc/passwd", true},
		{"/usr", true},
		{"/usr/bin", true},
		{"/bin/sh", true},
		{"/sys/class", true},
		{"/proc/cpuinfo", true},
		{"~/.ssh", true},
		{"~/.ssh/id_rsa", true},
		{"/home/testuser/.ssh/config", true},
		{"~/.gnupg", true},
		{"~/.aws", true},
		{"~/.aws/credentials", true},
		{"~/.azure", true},
		{"~/.config/gcloud", true},
		{"~/.password-store", true},
		{"~/.vault-token", true},
		{"~/.config/myapp", false},
		{"~/.cache/myapp", false},
		{"~/.cargo", false},
		{"/opt/myapp", false},
	}

	for _, tt := range tests {
		got := IsSensitivePath(tt.path, homeDir)
		if got != tt.sensitive {
			t.Errorf("IsSensitivePath(%q) = %v, want %v", tt.path, got, tt.sensitive)
		}
	}
}

func TestFilterSensitiveWrites(t *testing.T) {
	homeDir := "/home/testuser"
	bindsRW := []string{
		"~/.config/myapp",
		"~/.ssh",
		"/etc/custom",
		"~/.cache/pip",
	}

	safeRW, alerts := FilterSensitiveWrites(bindsRW, homeDir)

	wantSafe := []string{"~/.config/myapp", "~/.cache/pip"}
	if len(safeRW) != len(wantSafe) {
		t.Fatalf("len(safeRW) = %d, want %d: %v", len(safeRW), len(wantSafe), safeRW)
	}
	for i := range wantSafe {
		if safeRW[i] != wantSafe[i] {
			t.Errorf("safeRW[%d] = %q, want %q", i, safeRW[i], wantSafe[i])
		}
	}

	if len(alerts) != 2 {
		t.Errorf("len(alerts) = %d, want 2: %v", len(alerts), alerts)
	}
}
