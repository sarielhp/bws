package config

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func rwHosts(binds []BindEntry) []string {
	var hosts []string
	for _, b := range binds {
		hosts = append(hosts, b.Host)
	}
	return hosts
}

func TestMergeRestrictsLocalRW(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, d := range []string{".local/share/app", "work"} {
		if err := os.MkdirAll(filepath.Join(home, d), 0755); err != nil {
			t.Fatal(err)
		}
	}
	global := &Config{BindsRO: []BindEntry{{Host: "~/.local"}}}
	cases := []struct {
		host    string
		allowed bool
	}{
		{"~/.local", false},          // same path as a global read-only bind
		{"@@HOME@@/.local/", false},  // same path, different spelling
		{"~/.local/share/app", true}, // narrow hole under a read-only dir
		{"~", true},                  // parent: deeper read-only bind still wins
		{"~/work", true},             // unrelated path
		{"/usr/lib", false},          // under a system root
		{"/etc", false},              // system root itself
		{"/", false},                 // covers every system root
		{"relative/dir", true},       // workspace-relative
	}
	for _, tc := range cases {
		local := &Config{BindsRW: []BindEntry{{Host: tc.host}}}
		got := Merge(global, local)
		kept := slices.Contains(rwHosts(got.BindsRW), tc.host)
		if kept != tc.allowed {
			t.Errorf("%s: kept=%v, want %v", tc.host, kept, tc.allowed)
		}
		if !tc.allowed && len(got.RejectedBinds) != 1 {
			t.Errorf("%s: rejection not reported: %v", tc.host, got.RejectedBinds)
		}
	}
}

func TestMergeKeepsGlobalRW(t *testing.T) {
	global := &Config{
		BindsRO: []BindEntry{{Host: "/opt/x"}},
		BindsRW: []BindEntry{{Host: "/opt/x"}, {Host: "/usr/local/share/x"}},
	}
	got := Merge(global, &Config{})
	if len(got.BindsRW) != 2 || len(got.RejectedBinds) != 0 {
		t.Fatalf("global binds must not be filtered: %+v", got)
	}
}

func TestMergeRestrictsLocalRWWithoutGlobal(t *testing.T) {
	got := Merge(nil, &Config{BindsRW: []BindEntry{{Host: "/usr"}}})
	if len(got.BindsRW) != 0 {
		t.Fatalf("system root allowed with no global config: %v", got.BindsRW)
	}
}

func TestRestrictRWFollowsSymlinks(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "sneaky")
	if err := os.Symlink("/usr", link); err != nil {
		t.Fatal(err)
	}
	kept, rejected := RestrictRW(nil, []BindEntry{{Host: link}})
	if len(kept) != 0 || len(rejected) != 1 {
		t.Fatalf("symlink to /usr allowed: kept=%v", kept)
	}
}
