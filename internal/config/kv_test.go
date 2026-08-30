package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigKVSetGetUnset(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "config.jsonc")

	initial := `{
  // A comment
  "max_file_count": 10000,
  "features": {
    "enable_ssh": true
  }
}`
	if err := os.WriteFile(confPath, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	// Test Setting alias: enable_proxy -> features.enable_proxy
	if err := SetConfigKV(confPath, "enable_proxy", "true"); err != nil {
		t.Fatalf("SetConfigKV failed: %v", err)
	}

	val, err := GetConfigKV(confPath, "enable_proxy")
	if err != nil {
		t.Fatalf("GetConfigKV failed: %v", err)
	}
	if val != "true" {
		t.Errorf("expected true, got %s", val)
	}

	// Test setting top-level number
	if err := SetConfigKV(confPath, "max_file_count", "25000"); err != nil {
		t.Fatalf("SetConfigKV max_file_count failed: %v", err)
	}
	val, err = GetConfigKV(confPath, "max_file_count")
	if err != nil {
		t.Fatalf("GetConfigKV max_file_count failed: %v", err)
	}
	if val != "25000" {
		t.Errorf("expected 25000, got %s", val)
	}

	// Test unsetting
	if err := UnsetConfigKV(confPath, "enable_proxy"); err != nil {
		t.Fatalf("UnsetConfigKV failed: %v", err)
	}
	if _, err := GetConfigKV(confPath, "enable_proxy"); err == nil {
		t.Errorf("expected error getting unset key, got nil")
	}
}
