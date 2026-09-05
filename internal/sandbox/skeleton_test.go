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

func TestPrecreateMountpointsPinholeFile(t *testing.T) {
	homeDir := t.TempDir()
	notesDir := filepath.Join(homeDir, "notes")
	workDir := filepath.Join(notesDir, "06_verify")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	prefixFile := filepath.Join(notesDir, "prefix.tex")
	if err := os.WriteFile(prefixFile, []byte("prefix content"), 0644); err != nil {
		t.Fatal(err)
	}

	stageDir := filepath.Join(t.TempDir(), "stage")
	if err := os.MkdirAll(stageDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		BindsRO: []config.BindEntry{
			{Host: "../prefix.tex"},
		},
	}

	precreateMountpoints(cfg, stageDir, workDir, homeDir)

	// Verify that stageDir/notes/prefix.tex was created as a mountpoint file
	stagedPrefix := filepath.Join(stageDir, "notes", "prefix.tex")
	fi, err := os.Stat(stagedPrefix)
	if err != nil {
		t.Fatalf("expected staged mountpoint %s to exist: %v", stagedPrefix, err)
	}
	if fi.IsDir() {
		t.Errorf("expected %s to be a regular file mountpoint, but it is a directory", stagedPrefix)
	}
}

func TestDefaultTmuxSkeletonDirectives(t *testing.T) {
	requiredDirectives := []string{
		"set -g mouse on",
		"set -g history-limit 50000",
		"bind-key -n WheelUpPane",
		"bind-key -n WheelDownPane",
		"bind-key -n M-Left select-pane -L",
		"bind-key -n M-Right select-pane -R",
		"bind-key -n M-Up select-pane -U",
		"bind-key -n M-Down select-pane -D",
	}

	for _, directive := range requiredDirectives {
		if !strings.Contains(DefaultTmuxConf, directive) {
			t.Errorf("DefaultTmuxConf missing required directive %q", directive)
		}
	}
}

func TestStageHomePopulatesTmuxConf(t *testing.T) {
	cfg := &config.Config{}
	workDir := t.TempDir()

	stageDir, cleanup, err := StageHome(cfg, workDir)
	if err != nil {
		t.Fatalf("StageHome failed: %v", err)
	}
	defer cleanup()

	tmuxData, err := os.ReadFile(filepath.Join(stageDir, ".tmux.conf"))
	if err != nil {
		t.Fatalf("staged .tmux.conf missing: %v", err)
	}

	content := string(tmuxData)
	if !strings.Contains(content, "set -g mouse on") {
		t.Errorf("staged .tmux.conf missing 'set -g mouse on'")
	}
	if !strings.Contains(content, "M-Left select-pane -L") {
		t.Errorf("staged .tmux.conf missing M-Left pane navigation")
	}
}
