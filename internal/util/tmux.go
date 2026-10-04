package util

import (
	"os"
	"os/exec"
	"strings"
)

// HostTmuxState captures the host tmux window and pane attributes before bws changes them.
type HostTmuxState struct {
	PaneID     string
	WindowName string
	AutoRename bool
	PaneTitle  string
	Active     bool
}

// IsInsideTmux reports whether the current process is running inside a tmux session.
func IsInsideTmux() bool {
	return os.Getenv("TMUX") != ""
}

// SetHostTmuxTitle prepends "[BTS] " (or custom prefix) to the current host tmux window and pane title.
// It returns a cleanup function that restores the original window name, automatic-rename,
// and pane title when bws exits.
func SetHostTmuxTitle() (func(), error) {
	if !IsInsideTmux() {
		return func() {}, nil
	}

	out, err := exec.Command("tmux", "display-message", "-p", "#{pane_id}\t#W\t#{automatic-rename}\t#{pane_title}").Output()
	if err != nil {
		return func() {}, err
	}

	state := parseTmuxOutput(string(out))
	if state == nil {
		return func() {}, nil
	}

	prefix := "[BTS] "
	if custom := os.Getenv("BWS_PANE_PREFIX"); custom != "" {
		prefix = custom
	}

	newWinName := formatPrefixedTitle(state.WindowName, prefix)
	newPaneTitle := formatPrefixedTitle(state.PaneTitle, prefix)

	_ = exec.Command("tmux", "rename-window", "-t", state.PaneID, newWinName).Run()
	_ = exec.Command("tmux", "select-pane", "-t", state.PaneID, "-T", newPaneTitle).Run()

	cleanup := func() {
		if !state.Active {
			return
		}
		state.Active = false
		if state.AutoRename {
			_ = exec.Command("tmux", "set-window-option", "-t", state.PaneID, "automatic-rename", "on").Run()
		} else {
			_ = exec.Command("tmux", "rename-window", "-t", state.PaneID, state.WindowName).Run()
		}
		_ = exec.Command("tmux", "select-pane", "-t", state.PaneID, "-T", state.PaneTitle).Run()
	}

	return cleanup, nil
}

func parseTmuxOutput(out string) *HostTmuxState {
	parts := strings.SplitN(strings.TrimSpace(out), "\t", 4)
	if len(parts) < 4 || parts[0] == "" {
		return nil
	}
	return &HostTmuxState{
		PaneID:     parts[0],
		WindowName: parts[1],
		AutoRename: parts[2] == "1",
		PaneTitle:  parts[3],
		Active:     true,
	}
}

func formatPrefixedTitle(current, prefix string) string {
	if !strings.HasPrefix(current, prefix) && !strings.HasPrefix(current, "[BTS]") && !strings.HasPrefix(current, "[BWS]") {
		return prefix + current
	}
	return current
}
