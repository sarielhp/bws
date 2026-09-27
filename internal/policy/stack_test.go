package policy

import (
	"testing"

	"bws/internal/config"
	"bws/internal/stack"
)

func TestApplyStackSuccess(t *testing.T) {
	stk, err := stack.Get("go-agent")
	if err != nil {
		t.Fatalf("failed to get go-agent stack: %v", err)
	}
	digest, err := stack.Digest(stk)
	if err != nil {
		t.Fatalf("failed to get digest: %v", err)
	}

	cfg := &config.Config{
		Stack: "go-agent",
		ReviewedStack: &config.ProfileApproval{
			Source: "embedded",
			SHA256: digest,
		},
		Profiles: []string{"copilot"},
		Env: map[string]string{
			"USER_SETTING": "1",
		},
	}

	if err := ApplyStack(cfg, "."); err != nil {
		t.Fatalf("ApplyStack failed: %v", err)
	}

	// Verify go-agent profiles prepended
	if len(cfg.Profiles) < 3 {
		t.Fatalf("expected at least 3 profiles, got: %v", cfg.Profiles)
	}
	if cfg.Profiles[0] != "go-dev" || cfg.Profiles[1] != "secure-agent" || cfg.Profiles[2] != "copilot" {
		t.Errorf("unexpected profile ordering: %v", cfg.Profiles)
	}

	if cfg.Env["BWS_ACTIVE_STACK"] != "go-agent" {
		t.Errorf("expected BWS_ACTIVE_STACK=go-agent, got %q", cfg.Env["BWS_ACTIVE_STACK"])
	}
	if cfg.Env["USER_SETTING"] != "1" {
		t.Errorf("expected USER_SETTING=1 preserved, got %q", cfg.Env["USER_SETTING"])
	}
}

func TestApplyStackDigestMismatch(t *testing.T) {
	cfg := &config.Config{
		Stack: "go-agent",
		ReviewedStack: &config.ProfileApproval{
			Source: "embedded",
			SHA256: "0000000000000000000000000000000000000000000000000000000000000000",
		},
	}

	err := ApplyStack(cfg, ".")
	if err == nil {
		t.Fatalf("expected error on digest mismatch, got nil")
	}
}

func TestApplyStackMissing(t *testing.T) {
	cfg := &config.Config{
		Stack: "non-existent-stack",
	}

	err := ApplyStack(cfg, ".")
	if err == nil {
		t.Fatalf("expected error on missing stack, got nil")
	}
}
