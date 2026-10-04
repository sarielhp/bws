package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Regression: appending to an already-populated array must place the new
// element on its own indented line, not joined to the previous element.
func TestAddBindArrayElementOwnLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".bws", "config.jsonc")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	initial := "{\n  \"binds_ro\": [\n    [\"a\", \"b\"],\n    [\"c\", \"d\"]\n  ]\n}"
	if err := WriteTrustedFile(path, []byte(initial)); err != nil {
		t.Fatal(err)
	}
	if err := AddBindArrayElement(path, "binds_ro", `"/x"`); err != nil {
		t.Fatalf("append: %v", err)
	}
	data, _ := os.ReadFile(path)
	out := string(data)
	if strings.Contains(out, `],"/x"`) || strings.Contains(out, `], "/x"`) {
		t.Fatalf("new element joined to previous line:\n%s", out)
	}
	if !strings.Contains(out, "\n    \"/x\"") {
		t.Fatalf("new element not on its own indented line:\n%s", out)
	}
	cfg, err := Parse(data, path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(cfg.BindsRO) != 3 || cfg.BindsRO[2].Host != "/x" {
		t.Fatalf("unexpected rows: %+v", cfg.BindsRO)
	}
}

// The same guarantee for the plain string-array appender used by path/mask/etc.
func TestAddArrayElementOwnLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".bws", "config.jsonc")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	initial := "{\n  \"pass_env\": [\n    \"A\",\n    \"B\"\n  ]\n}"
	if err := WriteTrustedFile(path, []byte(initial)); err != nil {
		t.Fatal(err)
	}
	if err := AddArrayElement(path, "pass_env", "C"); err != nil {
		t.Fatalf("append: %v", err)
	}
	data, _ := os.ReadFile(path)
	out := string(data)
	if strings.Contains(out, `"B","C"`) || strings.Contains(out, `"B", "C"`) {
		t.Fatalf("new element joined to previous line:\n%s", out)
	}
	if !strings.Contains(out, "\n    \"C\"") {
		t.Fatalf("new element not on its own indented line:\n%s", out)
	}
}

// A compact hand-written array stays compact: the new element is joined inline
// rather than forced onto its own line.
func TestAddArrayElementCompactStaysCompact(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".bws", "config.jsonc")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := WriteTrustedFile(path, []byte("{\"mask\":[\"a\",\"b\"]}\n")); err != nil {
		t.Fatal(err)
	}
	if err := AddArrayElement(path, "mask", "c"); err != nil {
		t.Fatalf("append: %v", err)
	}
	data, _ := os.ReadFile(path)
	out := string(data)
	if strings.Contains(out, `"b",`+"\n") {
		t.Fatalf("compact array was broken onto a new line:\n%s", out)
	}
	if !strings.Contains(out, `"a","b","c"`) && !strings.Contains(out, `"a","b", "c"`) {
		t.Fatalf("new element not appended inline:\n%s", out)
	}
	cfg, err := Parse(data, path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(cfg.Mask) != 3 {
		t.Fatalf("mask=%v", cfg.Mask)
	}
}

// Appending to the packed default asset (whose elements are compact rows and
// whose binds_ro holds only a comment) must stay valid and mountable.
func TestAddBindArrayElementOnGeneratedDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".bws", "config.jsonc")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := WriteTrustedFile(path, []byte(generateDefaultConfig())); err != nil {
		t.Fatal(err)
	}
	for _, host := range []string{"/tmp/bin/x", "/tmp/bin/y"} {
		if err := AddBindArrayElement(path, "binds_ro", `"`+host+`"`); err != nil {
			t.Fatalf("append %s: %v", host, err)
		}
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), `"],"/tmp/bin/x"`) {
		t.Fatalf("entry joined to previous row:\n%s", data)
	}
	cfg, err := Parse(data, path)
	if err != nil {
		t.Fatalf("generated+appended config no longer parses: %v", err)
	}
	hosts := map[string]bool{}
	for _, b := range cfg.BindsRO {
		hosts[b.Host] = true
	}
	if !hosts["/tmp/bin/x"] || !hosts["/tmp/bin/y"] {
		t.Fatalf("appended entries missing: %v", hosts)
	}
}
