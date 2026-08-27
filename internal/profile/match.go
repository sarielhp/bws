package profile

import (
	"bufio"
	"os"
	"runtime"
	"strings"
)

// MatchCondition defines criteria for applying OS/environment rules.
type MatchCondition struct {
	Distro     []string `json:"distro,omitempty"`
	WSL        *bool    `json:"wsl,omitempty"`
	Arch       []string `json:"arch,omitempty"`
	ProbePaths []string `json:"probe_paths,omitempty"`
}

// ProfileRule contains conditional overrides when matching rules apply.
type ProfileRule struct {
	Match   *MatchCondition   `json:"match,omitempty"`
	Path    []string          `json:"path,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	BindsRW [][]string        `json:"binds_rw,omitempty"`
	BindsRO [][]string        `json:"binds_ro,omitempty"`
}

// MatchContext represents the detected host operating environment.
type MatchContext struct {
	OS     string
	Distro string
	WSL    bool
	Arch   string
}

// DetectMatchContext probes the host system to determine OS, Distro, WSL, and Arch.
func DetectMatchContext() MatchContext {
	ctx := MatchContext{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}

	if _, err := os.Stat("/run/WSL"); err == nil {
		ctx.WSL = true
	} else if os.Getenv("WSL_DISTRO_NAME") != "" {
		ctx.WSL = true
	} else if _, err := os.Stat("/proc/sys/fs/binfmt_misc/WSLInterop"); err == nil {
		ctx.WSL = true
	}

	ctx.Distro = detectLinuxDistro()
	return ctx
}

func detectLinuxDistro() string {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var id, idLike string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "ID=") {
			id = strings.Trim(strings.TrimPrefix(line, "ID="), "\"'")
		} else if strings.HasPrefix(line, "ID_LIKE=") {
			idLike = strings.Trim(strings.TrimPrefix(line, "ID_LIKE="), "\"'")
		}
	}

	if id != "" {
		return strings.ToLower(id)
	}
	if idLike != "" {
		parts := strings.Fields(idLike)
		if len(parts) > 0 {
			return strings.ToLower(parts[0])
		}
	}
	return ""
}

// MatchRule checks if a rule matches the provided host context.
func MatchRule(rule ProfileRule, ctx MatchContext) bool {
	if rule.Match == nil {
		return true
	}
	m := rule.Match

	if m.WSL != nil && *m.WSL != ctx.WSL {
		return false
	}

	if len(m.Arch) > 0 {
		matchedArch := false
		for _, a := range m.Arch {
			if strings.EqualFold(a, ctx.Arch) {
				matchedArch = true
				break
			}
		}
		if !matchedArch {
			return false
		}
	}

	if len(m.Distro) > 0 {
		matchedDistro := false
		for _, d := range m.Distro {
			if strings.EqualFold(d, ctx.Distro) {
				matchedDistro = true
				break
			}
		}
		if !matchedDistro {
			return false
		}
	}

	if len(m.ProbePaths) > 0 {
		for _, p := range m.ProbePaths {
			if _, err := os.Stat(p); err != nil {
				return false
			}
		}
	}

	return true
}
