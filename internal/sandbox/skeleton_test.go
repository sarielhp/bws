package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bws/internal/config"
)

func TestStageHomeLayering(t *testing.T) {
	tmpDir := t.TempDir()

	// Mock global skeleton directory
	mockGlobalSkel := filepath.Join(tmpDir, "global_skel")
	os.MkdirAll(mockGlobalSkel, 0755)
	os.WriteFile(filepath.Join(mockGlobalSkel, ".tmux.conf"), []byte("# global tmux"), 0644)
	os.WriteFile(filepath.Join(mockGlobalSkel, ".bashrc"), []byte("# global bashrc\n"), 0644)

	// Mock project directory with .bws/skeleton
	projectDir := filepath.Join(tmpDir, "myproject")
	mockLocalSkel := filepath.Join(projectDir, ".bw", "skeleton")
	os.MkdirAll(mockLocalSkel, 0755)
	os.WriteFile(filepath.Join(mockLocalSkel, ".bashrc"), []byte("# local project bashrc\nalias project_cmd='echo hi'\n"), 0644)
	os.WriteFile(filepath.Join(mockLocalSkel, ".project_rc"), []byte("PROJECT_ENV=1"), 0644)

	// Test staging with temporary stage dir
	stageDir, err := os.MkdirTemp("", "stage_test_*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(stageDir)

	// Copy global
	if err := copyDirectoryContents(mockGlobalSkel, stageDir); err != nil {
		t.Fatalf("copyDirectoryContents global failed: %v", err)
	}
	// Overlay local
	if err := copyDirectoryContents(mockLocalSkel, stageDir); err != nil {
		t.Fatalf("copyDirectoryContents local failed: %v", err)
	}

	// Dynamic append
	cfg := &config.Config{
		Env: map[string]string{
			"GOPATH": "/test/go",
		},
	}
	if err := appendDynamicConfig(cfg, filepath.Join(stageDir, ".bashrc")); err != nil {
		t.Fatalf("appendDynamicConfig failed: %v", err)
	}

	// Verify .tmux.conf came from global
	tmuxData, err := os.ReadFile(filepath.Join(stageDir, ".tmux.conf"))
	if err != nil || !strings.Contains(string(tmuxData), "# global tmux") {
		t.Errorf("expected global .tmux.conf content, got: %s", string(tmuxData))
	}

	// Verify .project_rc came from local
	projData, err := os.ReadFile(filepath.Join(stageDir, ".project_rc"))
	if err != nil || !strings.Contains(string(projData), "PROJECT_ENV=1") {
		t.Errorf("expected local .project_rc content, got: %s", string(projData))
	}

	// Verify .bashrc was overlaid by local and has dynamic append
	bashrcData, err := os.ReadFile(filepath.Join(stageDir, ".bashrc"))
	if err != nil {
		t.Fatalf("reading staged .bashrc failed: %v", err)
	}
	bashrcStr := string(bashrcData)
	if !strings.Contains(bashrcStr, "project_cmd") {
		t.Errorf("expected local project bashrc alias, got:\n%s", bashrcStr)
	}
	if !strings.Contains(bashrcStr, `export GOPATH="/test/go"`) {
		t.Errorf("expected GOPATH dynamic appendix, got:\n%s", bashrcStr)
	}
}
