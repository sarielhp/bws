package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bws/internal/config"
)

func setupTestWorkspace(t *testing.T) (string, func()) {
	t.Helper()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir to tempDir: %v", err)
	}
	cleanup := func() {
		_ = os.Chdir(oldWd)
	}
	return tmpDir, cleanup
}

func hasBindHost(entries []config.BindEntry, host string) bool {
	for _, b := range entries {
		if b.Host == host {
			return true
		}
	}
	return false
}

func TestHandleBindAddDefaultRO(t *testing.T) {
	tmpDir, cleanup := setupTestWorkspace(t)
	defer cleanup()

	hostFile := filepath.Join(t.TempDir(), "target.txt")
	if err := os.WriteFile(hostFile, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := runBindAdd(hostFile, "", false, false, true); err != nil {
		t.Fatalf("runBindAdd failed: %v", err)
	}

	cfgPath := filepath.Join(tmpDir, ".bws", "config.jsonc")
	cfg, err := config.LoadFile(cfgPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if !hasBindHost(cfg.BindsRO, hostFile) {
		t.Errorf("expected hostFile %s in BindsRO, got: %v", hostFile, cfg.BindsRO)
	}
	if hasBindHost(cfg.BindsRW, hostFile) {
		t.Errorf("did not expect hostFile %s in BindsRW, got: %v", hostFile, cfg.BindsRW)
	}

	if err := runBindDel(hostFile, false, true); err != nil {
		t.Fatalf("runBindDel failed: %v", err)
	}
	cfgAfter, _ := config.LoadFile(cfgPath)
	if hasBindHost(cfgAfter.BindsRO, hostFile) {
		t.Errorf("expected hostFile removed from BindsRO, got: %v", cfgAfter.BindsRO)
	}
}

func TestHandleBindAddRWOptIn(t *testing.T) {
	tmpDir, cleanup := setupTestWorkspace(t)
	defer cleanup()

	hostFile := filepath.Join(t.TempDir(), "target_rw.txt")
	if err := os.WriteFile(hostFile, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := runBindAdd(hostFile, "", true, false, true); err != nil {
		t.Fatalf("runBindAdd --rw failed: %v", err)
	}

	cfgPath := filepath.Join(tmpDir, ".bws", "config.jsonc")
	cfg, err := config.LoadFile(cfgPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if !hasBindHost(cfg.BindsRW, hostFile) {
		t.Errorf("expected hostFile in BindsRW, got: %v", cfg.BindsRW)
	}
	if hasBindHost(cfg.BindsRO, hostFile) {
		t.Errorf("did not expect hostFile in BindsRO, got: %v", cfg.BindsRO)
	}
}

func TestHandleBindAddExternalSymlinkResolution(t *testing.T) {
	tmpDir, cleanup := setupTestWorkspace(t)
	defer cleanup()

	extDir := t.TempDir()
	realFile := filepath.Join(extDir, "real.txt")
	if err := os.WriteFile(realFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	symlinkFile := filepath.Join(tmpDir, "link_to_ext.txt")
	if err := os.Symlink(realFile, symlinkFile); err != nil {
		t.Fatal(err)
	}

	if err := runBindAdd("link_to_ext.txt", "", false, false, true); err != nil {
		t.Fatalf("runBindAdd failed on symlink: %v", err)
	}

	cfgPath := filepath.Join(tmpDir, ".bws", "config.jsonc")
	cfg, err := config.LoadFile(cfgPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if !hasBindHost(cfg.BindsRO, realFile) {
		t.Errorf("expected canonical target %s in BindsRO, got %v", realFile, cfg.BindsRO)
	}
}

func TestHandleBindAddExternalSymlinkUnderHome(t *testing.T) {
	tmpDir, cleanup := setupTestWorkspace(t)
	defer cleanup()

	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	homeTarget := filepath.Join(fakeHome, "important.data")
	if err := os.WriteFile(homeTarget, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	symlinkFile := filepath.Join(tmpDir, "link_to_home.data")
	if err := os.Symlink(homeTarget, symlinkFile); err != nil {
		t.Fatal(err)
	}

	if err := runBindAdd("link_to_home.data", "", false, false, true); err != nil {
		t.Fatalf("runBindAdd failed: %v", err)
	}

	cfgPath := filepath.Join(tmpDir, ".bws", "config.jsonc")
	rawContent, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("failed to read raw config: %v", err)
	}
	expectedToken := config.HomeToken + "/important.data"
	if !strings.Contains(string(rawContent), expectedToken) {
		t.Errorf("expected tokenized target %s in raw config, got: %s", expectedToken, string(rawContent))
	}

	cfg, err := config.LoadFile(cfgPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if !hasBindHost(cfg.BindsRO, homeTarget) {
		t.Errorf("expected expanded home target %s in BindsRO, got %v", homeTarget, cfg.BindsRO)
	}

	if err := runBindDel("link_to_home.data", false, true); err != nil {
		t.Fatalf("runBindDel via symlink name failed: %v", err)
	}
	rawAfter, _ := os.ReadFile(cfgPath)
	if strings.Contains(string(rawAfter), expectedToken) {
		t.Errorf("expected tokenized target removed from raw config, got %s", string(rawAfter))
	}
}

func TestHandleBindAddInternalSymlinkSkip(t *testing.T) {
	tmpDir, cleanup := setupTestWorkspace(t)
	defer cleanup()

	subDir := filepath.Join(tmpDir, "internal_dir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	internalFile := filepath.Join(subDir, "internal.txt")
	if err := os.WriteFile(internalFile, []byte("inside"), 0644); err != nil {
		t.Fatal(err)
	}

	symlinkFile := filepath.Join(tmpDir, "link_inside.txt")
	if err := os.Symlink(internalFile, symlinkFile); err != nil {
		t.Fatal(err)
	}

	if err := runBindAdd("link_inside.txt", "", false, false, true); err != nil {
		t.Fatalf("unexpected error on internal symlink: %v", err)
	}

	cfgPath := filepath.Join(tmpDir, ".bws", "config.jsonc")
	if _, err := os.Stat(cfgPath); !os.IsNotExist(err) {
		t.Errorf("config file should not be created when internal symlink is skipped")
	}
}

func TestHandleBindAddDanglingSymlinkError(t *testing.T) {
	tmpDir, cleanup := setupTestWorkspace(t)
	defer cleanup()

	danglingPath := filepath.Join(t.TempDir(), "non_existent_target.txt")
	symlinkFile := filepath.Join(tmpDir, "broken_link.txt")
	if err := os.Symlink(danglingPath, symlinkFile); err != nil {
		t.Fatal(err)
	}

	err := runBindAdd("broken_link.txt", "", false, false, true)
	if err == nil {
		t.Fatal("expected error for dangling symlink, got nil")
	}
	if !strings.Contains(err.Error(), "dangling") {
		t.Errorf("expected error to mention dangling, got: %v", err)
	}

	cfgPath := filepath.Join(tmpDir, ".bws", "config.jsonc")
	if _, statErr := os.Stat(cfgPath); !os.IsNotExist(statErr) {
		t.Errorf("config file should not be created on error")
	}
}
