package trace

import (
	"os"
	"strings"
	"testing"
)

func TestPathFiltering(t *testing.T) {
	homeDir := "/home/user"
	workDir := "/home/user/myproject"

	tests := []struct {
		path       string
		shouldHide bool
	}{
		{"/proc/cpuinfo", true},
		{"/sys/class/net", true},
		{"/dev/null", true},
		{"/usr/lib/libc.so", true},
		{"/bin/bash", true},
		{"/etc/ld.so.cache", true},
		{"/etc/resolv.conf", true},
		{"/tmp/tempfile.dat", true},
		{"/var/tmp/temp.dat", true},
		{"/home/user", true},
		{"/home/user/myproject", true},
		{"/home/user/myproject/src/main.go", true},
		{"/home/user/.config/myapp/config.json", false},
		{"/home/user/.gitconfig", false},
		{"/opt/custom/bin/tool", false},
	}

	for _, tt := range tests {
		got := ShouldFilterPath(tt.path, workDir, homeDir)
		if got != tt.shouldHide {
			t.Errorf("ShouldFilterPath(%q) = %v, want %v", tt.path, got, tt.shouldHide)
		}
	}
}

func TestCollapseAndClassify(t *testing.T) {
	homeDir := "/home/user"

	accesses := map[string]AccessMode{
		"/home/user/.config/myapp/settings.json":  AccessRead,
		"/home/user/.config/myapp/plugins/foo.so": AccessWrite, // turns ~/.config/myapp into RW
		"/home/user/.cache/myapp/temp.dat":        AccessWrite, // ~/.cache/myapp is RW
		"/home/user/.gitconfig":                   AccessRead,  // single dotfile RO
		"/home/user/.ssh/config":                  AccessRead,  // RO
		"/home/user/.ssh/known_hosts":             AccessRead,  // RO
		"/home/user/.gemini/session.json":         AccessWrite, // ~/.gemini is RW
		"/opt/custom/lib/liba.so":                 AccessRead,  // /opt/custom is RO
	}

	bindsRW, bindsRO := CollapseAndClassify(accesses, homeDir)

	wantRW := []string{
		"~/.cache/myapp",
		"~/.config/myapp",
		"~/.gemini",
	}

	wantRO := []string{
		"/opt/custom",
		"~/.gitconfig",
		"~/.ssh",
	}

	if strings.Join(bindsRW, ",") != strings.Join(wantRW, ",") {
		t.Errorf("bindsRW = %v, want %v", bindsRW, wantRW)
	}

	if strings.Join(bindsRO, ",") != strings.Join(wantRO, ",") {
		t.Errorf("bindsRO = %v, want %v", bindsRO, wantRO)
	}
}

func TestSocketAndFeatureDetection(t *testing.T) {
	os.Setenv("SSH_AUTH_SOCK", "/run/user/1000/keyring/ssh")
	defer os.Unsetenv("SSH_AUTH_SOCK")

	features := DetectedFeatures{}

	DetectSocketFeatures("AF_INET:{sa_family=AF_INET, sin_port=htons(443)}", &features)
	if !features.Net {
		t.Errorf("expected Net to be true for AF_INET")
	}

	DetectSocketFeatures("AF_UNIX:/run/user/1000/bus", &features)
	if !features.DBus {
		t.Errorf("expected DBus to be true for /run/user/1000/bus")
	}

	DetectSocketFeatures("AF_UNIX:/run/user/1000/keyring/ssh", &features)
	if !features.SSH {
		t.Errorf("expected SSH to be true for SSH_AUTH_SOCK match")
	}

	DetectSocketFeatures("AF_UNIX:/tmp/.X11-unix/X0", &features)
	if !features.X11 {
		t.Errorf("expected X11 to be true for .X11-unix")
	}

	DetectPathFeatures("/run/WSL/interop", &features)
	if !features.WSL {
		t.Errorf("expected WSL to be true for /run/WSL path")
	}
}

func TestAnalyzeTraceLines(t *testing.T) {
	lines := []string{
		`1001 openat(AT_FDCWD, "/etc/ld.so.cache", O_RDONLY|O_CLOEXEC) = 3`,
		`1001 openat(AT_FDCWD, "/usr/lib/libc.so.6", O_RDONLY|O_CLOEXEC) = 3`,
		`1001 openat(AT_FDCWD, "/home/user/.gitconfig", O_RDONLY) = 3`,
		`1001 openat(AT_FDCWD, "/home/user/.config/mytool/config.toml", O_RDONLY) = 4`,
		`1001 openat(AT_FDCWD, "/home/user/.config/mytool/state.db", O_RDWR|O_CREAT, 0644) = 5`,
		`1001 connect(3, {sa_family=AF_INET, sin_port=htons(443), sin_addr=inet_addr("93.184.216.34")}, 16) = 0`,
		`1001 connect(4, {sa_family=AF_UNIX, sun_path="/tmp/ssh-abcdef/agent.1234"}, 110) = 0`,
	}

	opts := TraceOptions{
		Command: []string{"mytool", "run"},
		WorkDir: "/home/user/workspace",
		HomeDir: "/home/user",
	}

	res := AnalyzeTraceLines(lines, opts)

	if !res.Features.Net {
		t.Errorf("expected res.Features.Net = true")
	}
	if !res.Features.SSH {
		t.Errorf("expected res.Features.SSH = true")
	}
	if res.Features.DBus || res.Features.X11 || res.Features.WSL {
		t.Errorf("unexpected features detected: %+v", res.Features)
	}

	if len(res.BindsRW) != 1 || res.BindsRW[0] != "~/.config/mytool" {
		t.Errorf("res.BindsRW = %v, want [~/.config/mytool]", res.BindsRW)
	}

	if len(res.BindsRO) != 1 || res.BindsRO[0] != "~/.gitconfig" {
		t.Errorf("res.BindsRO = %v, want [~/.gitconfig]", res.BindsRO)
	}
}

func TestToProfile(t *testing.T) {
	res := &TraceResult{
		Command: []string{"git", "fetch"},
		Features: DetectedFeatures{
			Net:  true,
			SSH:  true,
			DBus: false,
		},
		BindsRW: []string{"~/.config/git"},
		BindsRO: []string{"~/.gitconfig"},
	}

	p := res.ToProfile("git-tool")

	if p.Name != "git-tool" {
		t.Errorf("p.Name = %q, want 'git-tool'", p.Name)
	}
	if len(p.BindsRW) != 1 || p.BindsRW[0][0] != "~/.config/git" || p.BindsRW[0][1] != "@@HOME@@/.config/git" {
		t.Errorf("p.BindsRW = %+v", p.BindsRW)
	}
	if len(p.BindsRO) != 1 || p.BindsRO[0][0] != "~/.gitconfig" || p.BindsRO[0][1] != "@@HOME@@/.gitconfig" {
		t.Errorf("p.BindsRO = %+v", p.BindsRO)
	}
	if p.Features == nil || p.Features.EnableSSH == nil || !*p.Features.EnableSSH {
		t.Errorf("p.Features.EnableSSH should be true")
	}
}
