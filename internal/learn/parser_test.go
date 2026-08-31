package learn

import (
	"testing"
)

func TestParseTraceLine(t *testing.T) {
	tests := []struct {
		name        string
		line        string
		wantName    string
		wantPaths   []string
		wantMode    AccessMode
		wantSock    string
		wantSuccess bool
	}{
		{
			name:        "open read-only",
			line:        `1001 openat(AT_FDCWD, "/etc/hosts", O_RDONLY|O_CLOEXEC) = 3`,
			wantName:    "openat",
			wantPaths:   []string{"/etc/hosts"},
			wantMode:    AccessRead,
			wantSuccess: true,
		},
		{
			name:        "open read-write with create",
			line:        `1001 openat(AT_FDCWD, "/home/user/.config/app/state.db", O_RDWR|O_CREAT|O_CLOEXEC, 0644) = 4`,
			wantName:    "openat",
			wantPaths:   []string{"/home/user/.config/app/state.db"},
			wantMode:    AccessWrite,
			wantSuccess: true,
		},
		{
			name:        "creat call",
			line:        `1001 creat("/tmp/newfile.txt", 0644) = 5`,
			wantName:    "creat",
			wantPaths:   []string{"/tmp/newfile.txt"},
			wantMode:    AccessWrite,
			wantSuccess: true,
		},
		{
			name:        "unlink file",
			line:        `1001 unlink("/home/user/.cache/app/old.tmp") = 0`,
			wantName:    "unlink",
			wantPaths:   []string{"/home/user/.cache/app/old.tmp"},
			wantMode:    AccessWrite,
			wantSuccess: true,
		},
		{
			name:        "rename file",
			line:        `1001 rename("/tmp/a", "/tmp/b") = 0`,
			wantName:    "rename",
			wantPaths:   []string{"/tmp/a", "/tmp/b"},
			wantMode:    AccessWrite,
			wantSuccess: true,
		},
		{
			name:        "mkdir directory",
			line:        `1001 mkdir("/home/user/.config/app", 0755) = 0`,
			wantName:    "mkdir",
			wantPaths:   []string{"/home/user/.config/app"},
			wantMode:    AccessWrite,
			wantSuccess: true,
		},
		{
			name:        "newfstatat read",
			line:        `1001 newfstatat(AT_FDCWD, "/home/user/.gitconfig", {st_mode=S_IFREG|0644}, 0) = 0`,
			wantName:    "newfstatat",
			wantPaths:   []string{"/home/user/.gitconfig"},
			wantMode:    AccessRead,
			wantSuccess: true,
		},
		{
			name:        "connect inet",
			line:        `1001 connect(3, {sa_family=AF_INET, sin_port=htons(443), sin_addr=inet_addr("93.184.216.34")}, 16) = 0`,
			wantName:    "connect",
			wantSock:    `AF_INET:{sa_family=AF_INET, sin_port=htons(443), sin_addr=inet_addr("93.184.216.34")}`,
			wantMode:    AccessNone,
			wantSuccess: true,
		},
		{
			name:        "connect unix socket",
			line:        `1001 connect(4, {sa_family=AF_UNIX, sun_path="/run/user/1000/bus"}, 110) = 0`,
			wantName:    "connect",
			wantSock:    `AF_UNIX:/run/user/1000/bus`,
			wantMode:    AccessNone,
			wantSuccess: true,
		},
		{
			name:        "enoent failure",
			line:        `1001 stat("/nonexistent/path", 0x7ffd) = -1 ENOENT (No such file or directory)`,
			wantName:    "stat",
			wantPaths:   []string{"/nonexistent/path"},
			wantMode:    AccessRead,
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := ParseTraceLine(tt.line)
			if p == nil {
				t.Fatalf("expected non-nil parsed line for %q", tt.line)
			}
			if p.Name != tt.wantName {
				t.Errorf("p.Name = %q, want %q", p.Name, tt.wantName)
			}
			if p.Mode != tt.wantMode {
				t.Errorf("p.Mode = %v, want %v", p.Mode, tt.wantMode)
			}
			if p.Success != tt.wantSuccess {
				t.Errorf("p.Success = %v, want %v", p.Success, tt.wantSuccess)
			}
			if len(tt.wantPaths) > 0 {
				if len(p.Paths) != len(tt.wantPaths) {
					t.Errorf("p.Paths = %v, want %v", p.Paths, tt.wantPaths)
				} else {
					for i := range tt.wantPaths {
						if p.Paths[i] != tt.wantPaths[i] {
							t.Errorf("p.Paths[%d] = %q, want %q", i, p.Paths[i], tt.wantPaths[i])
						}
					}
				}
			}
			if tt.wantSock != "" && !startsWith(p.SockAddr, tt.wantSock) {
				t.Errorf("p.SockAddr = %q, want prefix %q", p.SockAddr, tt.wantSock)
			}
		})
	}
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
