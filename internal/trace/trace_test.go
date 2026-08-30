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
		name       string
		path       string
		pathDirs   []string
		shouldHide bool
	}{
		{"proc cpuinfo", "/proc/cpuinfo", nil, true},
		{"sys net", "/sys/class/net", nil, true},
		{"dev null", "/dev/null", nil, true},
		{"usr libc", "/usr/lib/libc.so", nil, true},
		{"bin bash", "/bin/bash", nil, true},
		{"etc ld.so.cache", "/etc/ld.so.cache", nil, true},
		{"etc resolv.conf", "/etc/resolv.conf", nil, true},
		{"etc hosts", "/etc/hosts", nil, true},
		{"etc fonts", "/etc/fonts/fonts.conf", nil, true},
		{"etc custom", "/etc/custom/app.conf", nil, true},
		{"tmp file", "/tmp/tempfile.dat", nil, true},
		{"var tmp file", "/var/tmp/temp.dat", nil, true},
		{"home root", "/home/user", nil, true},
		{"workspace root", "/home/user/myproject", nil, true},
		{"workspace file", "/home/user/myproject/src/main.go", nil, true},
		{"user local bin dir", "/home/user/.local/bin", nil, true},
		{"user local bin exec", "/home/user/.local/bin/black", nil, true},
		{"user bin dir", "/home/user/bin", nil, true},
		{"user bin exec", "/home/user/bin/helper", nil, true},
		{"user cargo bin dir", "/home/user/.cargo/bin", nil, true},
		{"user cargo bin exec", "/home/user/.cargo/bin/rustc", nil, true},
		{"user go bin dir", "/home/user/go/bin", nil, true},
		{"user go bin exec", "/home/user/go/bin/gopls", nil, true},
		{"user config file", "/home/user/.config/myapp/config.json", nil, false},
		{"user gitconfig", "/home/user/.gitconfig", nil, false},
		{"user cargo config", "/home/user/.cargo/config.toml", nil, false},
		{"user local share", "/home/user/.local/share/myapp/data", nil, false},
		{"opt tool not in PATH", "/opt/custom/bin/tool", nil, false},
		{"opt tool in custom PATH", "/opt/custom/bin/tool", []string{"/opt/custom/bin"}, true},
		{"opt lib with custom PATH", "/opt/custom/lib/liba.so", []string{"/opt/custom/bin"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldFilterPath(tt.path, workDir, homeDir, tt.pathDirs...)
			if got != tt.shouldHide {
				t.Errorf("ShouldFilterPath(%q) = %v, want %v", tt.path, got, tt.shouldHide)
			}
		})
	}
}

func TestAccessModeFiltering(t *testing.T) {
	homeDir := "/home/user"
	workDir := "/home/user/myproject"

	// Read on /etc should be hidden; write on /etc should NOT be hidden
	if !ShouldFilterAccess("/etc/myapp/config.conf", AccessRead, workDir, homeDir) {
		t.Errorf("expected read on /etc to be filtered")
	}
	if ShouldFilterAccess("/etc/myapp/config.conf", AccessWrite, workDir, homeDir) {
		t.Errorf("expected write on /etc NOT to be filtered")
	}

	// Read/probe on PATH executable should be hidden; write should NOT be hidden
	if !ShouldFilterAccess("/home/user/.local/bin/black", AccessRead, workDir, homeDir) {
		t.Errorf("expected read probe in PATH to be filtered")
	}
	if ShouldFilterAccess("/home/user/.local/bin/black", AccessWrite, workDir, homeDir) {
		t.Errorf("expected write in PATH NOT to be filtered")
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
		"/etc/fonts/fonts.conf":                   AccessRead,  // Read-only /etc should be ignored
		"/etc/custom/app.conf":                    AccessWrite, // Write to /etc should be captured
	}

	bindsRW, bindsRO := CollapseAndClassify(accesses, homeDir)

	wantRW := []string{
		"/etc/custom",
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
		`1001 openat(AT_FDCWD, "/etc/hosts", O_RDONLY) = 3`,
		`1001 openat(AT_FDCWD, "/etc/resolv.conf", O_RDONLY) = 3`,
		`1001 openat(AT_FDCWD, "/etc/fonts/fonts.conf", O_RDONLY) = 3`,
		`1001 openat(AT_FDCWD, "/usr/lib/libc.so.6", O_RDONLY|O_CLOEXEC) = 3`,
		`1001 stat("/home/user/.local/bin/mytool", 0x7ffd) = -1 ENOENT`,
		`1001 access("/home/user/bin/mytool", X_OK) = -1 ENOENT`,
		`1001 newfstatat(AT_FDCWD, "/home/user/.cargo/bin", {st_mode=S_IFDIR|0755}, 0) = 0`,
		`1001 openat(AT_FDCWD, "/home/user/.cargo/bin/rustc", O_RDONLY) = 3`,
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
