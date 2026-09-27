package cli

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"bws/internal/stack"

	"github.com/fatih/color"
	"golang.org/x/term"
)

// StackChoice represents a selectable item in the interactive stack selector.
type StackChoice struct {
	Stack    *stack.Stack
	Badge    string
	IsBasic  bool
	IsCustom bool
	Label    string
}

type keyAction int

const (
	actUp keyAction = iota
	actDown
	actSelect
	actCancel
	actJump
	actNone
)

// SelectStackInteractive presents an in-line ANSI selector with a live explanation pane.
func SelectStackInteractive(title string, items []StackChoice) (int, error) {
	if len(items) == 0 {
		return -1, fmt.Errorf("no items to select")
	}
	stdinFd := int(os.Stdin.Fd())
	stderrFd := int(os.Stderr.Fd())
	if !term.IsTerminal(stdinFd) || !term.IsTerminal(stderrFd) {
		return -1, fmt.Errorf("non-interactive terminal")
	}

	termWidth, termHeight, err := term.GetSize(stderrFd)
	if err != nil || termHeight < 11 || termWidth < 40 {
		return -1, fmt.Errorf("terminal dimensions too small for interactive selector")
	}

	oldState, err := term.MakeRaw(stdinFd)
	if err != nil {
		return -1, err
	}
	defer term.Restore(stdinFd, oldState)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	return runSelectorLoop(title, items, termWidth)
}

func runSelectorLoop(title string, items []StackChoice, termWidth int) (int, error) {
	selected := 0
	linesRendered := 0
	out := os.Stderr

	for {
		if linesRendered > 0 {
			fmt.Fprintf(out, "\x1b[%dA", linesRendered)
		}
		linesRendered = renderStackSelector(out, title, items, selected, termWidth)

		action, jumpIdx, err := readSelectorKey(os.Stdin)
		if err != nil {
			clearSelector(out, linesRendered)
			return -1, err
		}

		switch action {
		case actCancel:
			clearSelector(out, linesRendered)
			return -1, fmt.Errorf("cancelled; no changes written")
		case actSelect:
			clearSelector(out, linesRendered)
			printSelectionConfirmation(out, items[selected])
			return selected, nil
		case actUp:
			selected = (selected - 1 + len(items)) % len(items)
		case actDown:
			selected = (selected + 1) % len(items)
		case actJump:
			if jumpIdx >= 0 && jumpIdx < len(items) {
				selected = jumpIdx
			}
		}
	}
}

func printSelectionConfirmation(w io.Writer, item StackChoice) {
	check := color.New(color.FgHiGreen, color.Bold).Sprint("✓")
	if item.Stack != nil {
		name := color.New(color.FgHiCyan, color.Bold).Sprint(item.Stack.Name)
		fmt.Fprintf(w, "%s Selected stack: %s (%s)\n", check, name, item.Stack.Title)
	} else {
		lbl := color.New(color.FgHiCyan, color.Bold).Sprint(item.Label)
		fmt.Fprintf(w, "%s Selected: %s\n", check, lbl)
	}
}

func clearSelector(w io.Writer, lines int) {
	if lines <= 0 {
		return
	}
	fmt.Fprintf(w, "\x1b[%dA\x1b[J", lines)
}

func readSelectorKey(r io.Reader) (keyAction, int, error) {
	var buf [3]byte
	n, err := r.Read(buf[:])
	if err != nil {
		return actCancel, 0, err
	}
	if n == 1 {
		switch buf[0] {
		case 3, 4, 27, 'q', 'Q':
			return actCancel, 0, nil
		case '\r', '\n':
			return actSelect, 0, nil
		case 'k', 'K':
			return actUp, 0, nil
		case 'j', 'J':
			return actDown, 0, nil
		default:
			if buf[0] >= '1' && buf[0] <= '9' {
				return actJump, int(buf[0] - '1'), nil
			}
			return actNone, 0, nil
		}
	}
	if n == 3 && buf[0] == 27 && buf[1] == '[' {
		switch buf[2] {
		case 'A':
			return actUp, 0, nil
		case 'B':
			return actDown, 0, nil
		}
	}
	return actNone, 0, nil
}
