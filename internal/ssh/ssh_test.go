package ssh

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKeyPath(t *testing.T) {
	t.Setenv("HOME", "/home/u")
	cases := map[string]string{
		".ssh/id_ed25519":                  "/home/u/.ssh/id_ed25519",
		"/home/u/.ssh/id_rsa":              "/home/u/.ssh/id_rsa",
		"/home/u/.sandbox/deploy_keys/o_r": "/home/u/.sandbox/deploy_keys/o_r",
	}
	for in, want := range cases {
		if got := keyPath(in); got != want {
			t.Errorf("keyPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEnsurePrivateDirTightensExisting(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".sandbox")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := ensurePrivateDir(dir); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0700 {
		t.Fatalf("mode = %o, want 700", fi.Mode().Perm())
	}
}
