package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeTemp(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.jsonc")
	if err := os.WriteFile(p, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSetConfigKVKeepsEnvKeyCase(t *testing.T) {
	p := writeTemp(t, `{"env": {"PATH": "/usr/bin"}}`)
	if err := SetConfigKV(p, "env.PATH", "/custom/bin"); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Env["PATH"] != "/custom/bin" || len(cfg.Env) != 1 {
		t.Fatalf("env = %v, want only PATH=/custom/bin", cfg.Env)
	}
	if NormalizeKey("Features.Enable_SSH") != "features.enable_ssh" {
		t.Fatal("non-env keys must still be case-insensitive")
	}
}

func TestSetConfigKVRejectsNonObjectParent(t *testing.T) {
	p := writeTemp(t, `{"features": "oops"}`)
	if err := SetConfigKV(p, "features.enable_ssh", "true"); err == nil {
		t.Fatal("expected error for non-object parent")
	}
	data, _ := os.ReadFile(p)
	if strings.Count(string(data), `"features"`) != 1 {
		t.Fatalf("duplicate key written: %s", data)
	}
}

func TestGetConfigKVUnquotesStrings(t *testing.T) {
	p := writeTemp(t, `{"sandbox_path": "/root/sb", "max_file_count": 5}`)
	for key, want := range map[string]string{"sandbox_path": "/root/sb", "max_file_count": "5"} {
		got, err := GetConfigKV(p, key)
		if err != nil || got != want {
			t.Errorf("GetConfigKV(%q) = %q, %v; want %q", key, got, err, want)
		}
	}
}

func TestAtomicWriteFileRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real.jsonc")
	link := filepath.Join(dir, "link.jsonc")
	if err := os.WriteFile(real, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if err := atomicWriteFile(link, []byte("new")); err == nil {
		t.Fatal("expected error writing through symlink")
	}
	data, _ := os.ReadFile(real)
	if string(data) != "old" {
		t.Fatalf("symlink target was overwritten: got %q, want %q", string(data), "old")
	}
}

func TestRemoveBindElementMatchesHostOnly(t *testing.T) {
	p := writeTemp(t, `{"binds_ro": [["/tmp/opencode/database", "/data"], "/data/x", "/data"]}`)
	found, err := RemoveBindElement(p, "binds_ro", "/data")
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	cfg, err := LoadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.BindsRO) != 2 || cfg.BindsRO[0].Host != "/tmp/opencode/database" {
		t.Fatalf("wrong entries removed: %+v", cfg.BindsRO)
	}
}

func TestGenerateDefaultConfigEscapesHome(t *testing.T) {
	t.Setenv("HOME", `/home/we"ird\x`)
	if _, err := Parse([]byte(generateDefaultConfig()), "gen.jsonc"); err != nil {
		t.Fatalf("generated config does not parse: %v", err)
	}
}

func TestConfigDirAbsoluteWithoutHome(t *testing.T) {
	t.Setenv("HOME", "")
	if !filepath.IsAbs(ConfigDir()) {
		t.Fatalf("ConfigDir() = %q, want absolute", ConfigDir())
	}
}

// fillPointers sets every nil pointer field reachable from v to a fresh value.
func fillPointers(v reflect.Value) {
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if f.Kind() != reflect.Pointer || !f.CanSet() {
			continue
		}
		f.Set(reflect.New(f.Type().Elem()))
		if f.Elem().Kind() == reflect.Struct {
			fillPointers(f.Elem())
		}
	}
}

// assertNoSharedPointers fails if any pointer field of a and b is the same.
func assertNoSharedPointers(t *testing.T, path string, a, b reflect.Value) {
	t.Helper()
	for i := 0; i < a.NumField(); i++ {
		fa, fb := a.Field(i), b.Field(i)
		if fa.Kind() != reflect.Pointer || fa.IsNil() {
			continue
		}
		name := path + "." + a.Type().Field(i).Name
		if fa.Pointer() == fb.Pointer() {
			t.Errorf("Clone shares pointer %s", name)
		}
		if fa.Elem().Kind() == reflect.Struct {
			assertNoSharedPointers(t, name, fa.Elem(), fb.Elem())
		}
	}
}

func TestCloneSharesNoPointers(t *testing.T) {
	c := &Config{}
	fillPointers(reflect.ValueOf(c).Elem())
	assertNoSharedPointers(t, "Config", reflect.ValueOf(c).Elem(), reflect.ValueOf(Clone(c)).Elem())
}
