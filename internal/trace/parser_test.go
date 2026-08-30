package trace

import (
	"testing"
)

func TestParseTraceLine(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantName  string
		wantPaths []string
		wantMode  AccessMode
		wantSock  string
	}{
		{
			name:      "openat read-only",
			line:      `1234 openat(AT_FDCWD, "/home/user/.config/app/conf.json", O_RDONLY|O_CLOEXEC) = 3`,
			wantName:  "openat",
			wantPaths: []string{"/home/user/.config/app/conf.json"},
			wantMode:  AccessRead,
		},
		{
			name:      "openat write/create",
			line:      `[pid 5678] openat(AT_FDCWD, "/home/user/.cache/app/cache.db", O_WRONLY|O_CREAT|O_TRUNC, 0666) = 4`,
			wantName:  "openat",
			wantPaths: []string{"/home/user/.cache/app/cache.db"},
			wantMode:  AccessWrite,
		},
		{
			name:      "creat write",
			line:      `creat("/home/user/.local/share/app/state", 0644) = 5`,
			wantName:  "creat",
			wantPaths: []string{"/home/user/.local/share/app/state"},
			wantMode:  AccessWrite,
		},
		{
			name:      "unlink write",
			line:      `unlink("/home/user/.cache/app/tmp.dat") = 0`,
			wantName:  "unlink",
			wantPaths: []string{"/home/user/.cache/app/tmp.dat"},
			wantMode:  AccessWrite,
		},
		{
			name:      "unlinkat write",
			line:      `unlinkat(AT_FDCWD, "/home/user/.cache/app/tmp.dat", 0) = 0`,
			wantName:  "unlinkat",
			wantPaths: []string{"/home/user/.cache/app/tmp.dat"},
			wantMode:  AccessWrite,
		},
		{
			name:      "rename write",
			line:      `rename("/home/user/a.txt", "/home/user/b.txt") = 0`,
			wantName:  "rename",
			wantPaths: []string{"/home/user/a.txt", "/home/user/b.txt"},
			wantMode:  AccessWrite,
		},
		{
			name:      "renameat write",
			line:      `renameat(AT_FDCWD, "/home/user/a.txt", AT_FDCWD, "/home/user/b.txt") = 0`,
			wantName:  "renameat",
			wantPaths: []string{"/home/user/a.txt", "/home/user/b.txt"},
			wantMode:  AccessWrite,
		},
		{
			name:      "mkdir write",
			line:      `mkdir("/home/user/.config/newapp", 0755) = 0`,
			wantName:  "mkdir",
			wantPaths: []string{"/home/user/.config/newapp"},
			wantMode:  AccessWrite,
		},
		{
			name:      "mkdirat write",
			line:      `mkdirat(AT_FDCWD, "/home/user/.config/newapp", 0755) = 0`,
			wantName:  "mkdirat",
			wantPaths: []string{"/home/user/.config/newapp"},
			wantMode:  AccessWrite,
		},
		{
			name:      "stat read",
			line:      `stat("/home/user/.gitconfig", {st_mode=S_IFREG|0644, st_size=123}) = 0`,
			wantName:  "stat",
			wantPaths: []string{"/home/user/.gitconfig"},
			wantMode:  AccessRead,
		},
		{
			name:      "newfstatat read",
			line:      `newfstatat(AT_FDCWD, "/home/user/.ssh/config", {st_mode=S_IFREG|0600}, 0) = 0`,
			wantName:  "newfstatat",
			wantPaths: []string{"/home/user/.ssh/config"},
			wantMode:  AccessRead,
		},
		{
			name:     "connect AF_INET",
			line:     `connect(3, {sa_family=AF_INET, sin_port=htons(443), sin_addr=inet_addr("1.1.1.1")}, 16) = 0`,
			wantName: "connect",
			wantMode: AccessNone,
			wantSock: `AF_INET:{sa_family=AF_INET, sin_port=htons(443), sin_addr=inet_addr("1.1.1.1")}, 16`,
		},
		{
			name:     "connect AF_UNIX dbus",
			line:     `connect(4, {sa_family=AF_UNIX, sun_path="/run/user/1000/bus"}, 110) = 0`,
			wantName: "connect",
			wantMode: AccessNone,
			wantSock: "AF_UNIX:/run/user/1000/bus",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseTraceLine(tt.line)
			if got == nil {
				t.Fatalf("ParseTraceLine(%q) returned nil", tt.line)
			}
			if got.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", got.Name, tt.wantName)
			}
			if len(got.Paths) != len(tt.wantPaths) {
				t.Errorf("Paths len = %d, want %d (%v vs %v)", len(got.Paths), len(tt.wantPaths), got.Paths, tt.wantPaths)
			} else {
				for i := range got.Paths {
					if got.Paths[i] != tt.wantPaths[i] {
						t.Errorf("Paths[%d] = %q, want %q", i, got.Paths[i], tt.wantPaths[i])
					}
				}
			}
			if got.Mode != tt.wantMode {
				t.Errorf("Mode = %v, want %v", got.Mode, tt.wantMode)
			}
			if tt.wantSock != "" && got.SockAddr != tt.wantSock {
				t.Errorf("SockAddr = %q, want %q", got.SockAddr, tt.wantSock)
			}
		})
	}
}

func TestExtractQuotedStrings(t *testing.T) {
	raw := `AT_FDCWD, "/path/with spaces/file.txt", O_RDONLY, "/second/path"`
	got := extractQuotedStrings(raw)
	if len(got) != 2 {
		t.Fatalf("expected 2 strings, got %d", len(got))
	}
	if got[0] != "/path/with spaces/file.txt" {
		t.Errorf("got[0] = %q", got[0])
	}
	if got[1] != "/second/path" {
		t.Errorf("got[1] = %q", got[1])
	}
}
