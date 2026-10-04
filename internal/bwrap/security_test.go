package bwrap

import (
	"net"
	"path/filepath"
	"testing"

	"bws/internal/config"
)

func TestDisabledSSHDoesNotBindAgent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	sock := filepath.Join(t.TempDir(), "agent.sock")
	listener, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	t.Setenv("SSH_AUTH_SOCK", sock)
	disabled := false
	cfg := &config.Config{Features: &config.FeaturesConfig{EnableSSH: &disabled}, PassEnv: []string{"SSH_AUTH_SOCK"}}
	args := BuildArgs(cfg, "<ephemeral staged home>", t.TempDir(), true, false)
	unset := false
	for i, arg := range args {
		if arg == "--bind" && i+1 < len(args) && args[i+1] == sock {
			t.Fatal("disabled agent socket mounted")
		}
		if arg == "--unsetenv" && i+1 < len(args) && args[i+1] == "SSH_AUTH_SOCK" {
			unset = true
		}
	}
	if !unset {
		t.Fatal("inherited agent environment not removed")
	}
}

func TestHomeAndWorkspaceMountOrdering(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	project := filepath.Join(home, "project")
	args := BuildArgs(&config.Config{}, "<ephemeral staged home>", project, true, false)
	tmpIndex, homeIndex, projectIndex := -1, -1, -1
	for i := 0; i+2 < len(args); i++ {
		if args[i] != "--bind" {
			continue
		}
		switch args[i+2] {
		case "/tmp":
			tmpIndex = i
		case home:
			homeIndex = i
		case project:
			projectIndex = i
		}
	}
	if tmpIndex < 0 || homeIndex <= tmpIndex || projectIndex <= homeIndex {
		t.Fatalf("mount order: tmp=%d home=%d workspace=%d", tmpIndex, homeIndex, projectIndex)
	}
}

func TestIsSystemOrHomeRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	blocked := []string{
		"/",
		".",
		"/usr",
		"/bin",
		"/sbin",
		"/usr/local",
		"/usr/bin",
		"/usr/sbin",
		"/etc",
		"/var",
		"/opt",
		home,
		filepath.Join(home, ".local"),
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, "bin"),
	}

	for _, p := range blocked {
		if !isSystemOrHomeRoot(p) {
			t.Errorf("expected isSystemOrHomeRoot(%q) = true, got false", p)
		}
	}

	allowed := []string{
		"/opt/quarto",
		"/opt/quarto/bin",
		filepath.Join(home, ".local", "share", "quarto"),
		"/custom/tools/quarto",
	}

	for _, p := range allowed {
		if isSystemOrHomeRoot(p) {
			t.Errorf("expected isSystemOrHomeRoot(%q) = false, got true", p)
		}
	}
}
