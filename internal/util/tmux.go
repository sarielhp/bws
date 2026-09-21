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

	out, err := exec.Command("tmux", "display-message", "-p", "#{pane_id}:#W:#{automatic-rename}:#{pane_title}").Output()
	if err != nil {
		return func() {}, err
	}

	parts := strings.SplitN(strings.TrimSpace(string(out)), ":", 4)
	if len(parts) < 4 || parts[0] == "" {
		return func() {}, nil
	}

	state := &HostTmuxState{
		PaneID:     parts[0],
		WindowName: parts[1],
		AutoRename: parts[2] == "1",
		PaneTitle:  parts[3],
		Active:     true,
	}

	prefix := "[BTS] "
	if custom := os.Getenv("BWS_PANE_PREFIX"); custom != "" {
		prefix = custom
	}

	newWinName := state.WindowName
	if !strings.HasPrefix(newWinName, prefix) && !strings.HasPrefix(newWinName, "[BTS]") && !strings.HasPrefix(newWinName, "[BWS]") {
		newWinName = prefix + newWinName
	}

	newPaneTitle := state.PaneTitle
	if !strings.HasPrefix(newPaneTitle, prefix) && !strings.HasPrefix(newPaneTitle, "[BTS]") && !strings.HasPrefix(newPaneTitle, "[BWS]") {
		newPaneTitle = prefix + newPaneTitle
	}

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
