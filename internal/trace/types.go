package trace

import (
	"bws/internal/config"
	"bws/internal/profile"
	"fmt"
	"strings"
)

// AccessMode bitmask representing read and/or write file operations.
type AccessMode int

const (
	AccessNone      AccessMode = 0
	AccessRead      AccessMode = 1 << 0
	AccessWrite     AccessMode = 1 << 1
	AccessReadWrite AccessMode = AccessRead | AccessWrite
)

// DetectedFeatures tracks sandbox isolation features detected during runtime tracing.
type DetectedFeatures struct {
	Net  bool `json:"net"`  // TCP/UDP network socket access
	DBus bool `json:"dbus"` // D-Bus session or system bus access
	SSH  bool `json:"ssh"`  // SSH agent authentication socket access
	X11  bool `json:"x11"`  // X11 display server socket access
	WSL  bool `json:"wsl"`  // WSL2 interop access
}

// ParsedSyscall contains information extracted from a single strace log line.
type ParsedSyscall struct {
	PID      int
	Name     string
	Paths    []string
	Mode     AccessMode
	SockAddr string
	RawArgs  string
	RetVal   int
	Success  bool
}

// TraceResult contains the aggregated output of dynamic runtime tracing.
type TraceResult struct {
	Command     []string              `json:"command"`
	ExitCode    int                   `json:"exit_code"`
	Features    DetectedFeatures      `json:"features"`
	BindsRW     []string              `json:"binds_rw"`
	BindsRO     []string              `json:"binds_ro"`
	AllAccesses map[string]AccessMode `json:"-"`
}

// TraceOptions configures the execution and parsing of a trace session.
type TraceOptions struct {
	Command []string
	WorkDir string
	HomeDir string
	Verbose bool
}

// ToProfile converts a TraceResult into a declarative capability Profile.
func (tr *TraceResult) ToProfile(name string) *profile.Profile {
	p := &profile.Profile{
		Name:        name,
		Description: fmt.Sprintf("Dynamic capability profile for %s", name),
		Tests: []profile.TestSpec{
			{
				Name: fmt.Sprintf("%s verification", name),
				Cmd:  tr.Command,
				Type: "smoke",
			},
		},
	}

	for _, rw := range tr.BindsRW {
		target := strings.Replace(rw, "~", "@@HOME@@", 1)
		p.BindsRW = append(p.BindsRW, []string{rw, target})
	}
	for _, ro := range tr.BindsRO {
		target := strings.Replace(ro, "~", "@@HOME@@", 1)
		p.BindsRO = append(p.BindsRO, []string{ro, target})
	}

	p.Features = &config.FeaturesConfig{}
	hasFeatures := false

	if tr.Features.SSH {
		t := true
		p.Features.EnableSSH = &t
		hasFeatures = true
	}
	if tr.Features.DBus {
		t := true
		p.Features.EnableDBus = &t
		hasFeatures = true
	}
	if tr.Features.X11 {
		t := true
		p.Features.EnableX11 = &t
		hasFeatures = true
	}
	if tr.Features.WSL {
		t := true
		p.Features.EnableWSL = &t
		hasFeatures = true
	}

	if !hasFeatures {
		p.Features = nil
	}

	return p
}
