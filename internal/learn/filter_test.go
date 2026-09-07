package learn

import (
	"path/filepath"
	"testing"
)

func TestShouldFilterAccess_AncestorFiltering(t *testing.T) {
	homeDir := "/home/testuser"
	courseDir := filepath.Join(homeDir, "course")
	notesDir := filepath.Join(courseDir, "notes")
	workDir := filepath.Join(notesDir, "06_verify")
	prefixFile := filepath.Join(notesDir, "prefix.tex")

	// 1. Ancestor directories should be filtered out
	if !ShouldFilterAccess(notesDir, AccessRead, workDir, homeDir) {
		t.Errorf("expected parent directory %s to be filtered, but it was not", notesDir)
	}
	if !ShouldFilterAccess(courseDir, AccessRead, workDir, homeDir) {
		t.Errorf("expected grandparent directory %s to be filtered, but it was not", courseDir)
	}

	// 2. Regular file in ancestor directory (pinhole candidate) must NOT be filtered
	if ShouldFilterAccess(prefixFile, AccessRead, workDir, homeDir) {
		t.Errorf("expected ancestor file %s to NOT be filtered, but it was filtered", prefixFile)
	}

	// 3. Workspace-internal file should be filtered (already mounted by CWD)
	workFile := filepath.Join(workDir, "verify.tex")
	if !ShouldFilterAccess(workFile, AccessRead, workDir, homeDir) {
		t.Errorf("expected workspace internal file %s to be filtered, but it was not", workFile)
	}
}

func TestShouldFilterAccess_SecretReadFiltering(t *testing.T) {
	homeDir := "/home/testuser"
	workDir := "/home/testuser/myproject"

	secretPaths := []string{
		"/home/testuser/.ssh",
		"/home/testuser/.ssh/id_rsa",
		"/home/testuser/.ssh/config",
		"/home/testuser/.config/gh/hosts.yml",
		"/home/testuser/.local/share/gh/hosts.yml",
		"/home/testuser/.local/state/gh/state.yml",
		"/home/testuser/.git-credentials",
		"/home/testuser/.config/git/credentials",
		"/home/testuser/.netrc",
		"/home/testuser/.config/netrc",
		"/home/testuser/.kube/config",
		"/home/testuser/.docker/config.json",
		"/home/testuser/.npmrc",
		"/home/testuser/.pypirc",
		"/home/testuser/.cargo/credentials.toml",
		"/home/testuser/.local/share/keyrings/default.keyring",
		"/home/testuser/.config/op/config",
		"/home/testuser/.terraform.d/credentials.tfrc.json",
	}

	for _, p := range secretPaths {
		if !ShouldFilterAccess(p, AccessRead, workDir, homeDir) {
			t.Errorf("expected secret path %s to be filtered on read, but it was allowed", p)
		}
	}

	nonSecretPaths := []string{
		"/home/testuser/.gitconfig",
		"/home/testuser/.cargo/config.toml",
		"/home/testuser/.config/app/settings.json",
	}

	for _, p := range nonSecretPaths {
		if ShouldFilterAccess(p, AccessRead, workDir, homeDir) {
			t.Errorf("expected non-secret path %s to NOT be filtered on read, but it was filtered", p)
		}
	}
}
