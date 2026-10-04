package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"bws/internal/config"

	"github.com/fatih/color"
)

// workspaceBannerWriter is the destination for workspace context banners. It is
// stderr so that diagnostic banners never pollute command output.
var workspaceBannerWriter io.Writer = os.Stderr

// workspaceBannerColor reports whether the banner may use ANSI color. It is set
// from the app's --no-color handling.
var workspaceBannerColor = true

// SetWorkspaceBannerColor records whether color output is enabled for banners.
func SetWorkspaceBannerColor(enabled bool) {
	workspaceBannerColor = enabled
}

// printWorkspaceBanner prints the workspace chain banner for a local command.
// Global targets have a single config file and print nothing here.
func printWorkspaceBanner(global bool) {
	if global {
		return
	}
	PrintWorkspaceBanner(workspaceBannerColor)
}

// PrintWorkspaceBanner prints the chain of .bws configurations on the upward
// search path before a mutating command, with the single write target marked by
// ">>>". The nearest directory is shown first. Color is applied only on a TTY
// and when color output is enabled.
func PrintWorkspaceBanner(colorEnabled bool) {
	if os.Getenv("BWS_NO_BANNER") != "" {
		return
	}
	cwd, err := os.Getwd()
	if err != nil {
		return
	}
	chain := config.WorkspaceCandidates(cwd)
	useColor := colorEnabled && IsInteractiveTTY(int(os.Stderr.Fd()))

	// Trivial case: a single candidate is always cwd, and always the target.
	if len(chain) == 1 {
		fmt.Fprintf(workspaceBannerWriter, "bws: %s %s\n",
			marker(useColor), formatChainDir(chain[0].Dir))
		return
	}

	fmt.Fprintln(workspaceBannerWriter, style(useColor, color.FgHiBlack, "bws workspace chain (nearest first):"))
	for _, c := range chain {
		role := candidateRole(c)
		line := formatChainDir(c.Dir)
		if c.Target {
			fmt.Fprintf(workspaceBannerWriter, "  %s %s %s\n",
				marker(useColor), style(useColor, color.FgCyan+color.Bold, line), style(useColor, color.FgHiBlack, "("+role+")"))
		} else {
			fmt.Fprintf(workspaceBannerWriter, "    %s %s\n", line, style(useColor, color.FgHiBlack, "("+role+")"))
		}
	}
}

func marker(useColor bool) string {
	if useColor {
		return color.New(color.FgGreen, color.Bold).Sprint(">>>")
	}
	return ">>>"
}

func style(useColor bool, attr color.Attribute, s string) string {
	if !useColor {
		return s
	}
	return color.New(attr).Sprint(s)
}

// candidateRole explains why a directory appears in the chain.
func candidateRole(c config.WorkspaceCandidate) string {
	switch {
	case c.Target && !c.Exists:
		return "target, created on write"
	case c.Target:
		return "target"
	case c.Exists:
		return "shadowed by a nearer workspace"
	default:
		return "no config here"
	}
}

// formatChainDir renders a directory as ~-relative when under home.
func formatChainDir(dir string) string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		if dir == home {
			return "~"
		}
		if strings.HasPrefix(dir, home+"/") {
			return "~" + strings.TrimPrefix(dir, home)
		}
	}
	return dir
}
