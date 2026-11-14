package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"bws/internal/config"
	"bws/internal/profile"
)

// ErrSetupCancelled reports that the user left the setup screen without saving.
var ErrSetupCancelled = errors.New("setup cancelled; nothing saved")

// SetupOptions selects how RunSetup obtains the selection.
type SetupOptions struct {
	TargetDir string
	// With, when set, replaces the screen: exactly these profiles (and what
	// they require) are checked. Access settings keep their current values.
	With []string
}

// RunSetup shows the setup screen (or applies the flag selection), then
// writes and approves the workspace configuration. The user reviewed the
// selection on screen, so the written file is trusted.
func RunSetup(opts SetupOptions) error {
	root, before, err := initDestination(opts.TargetDir)
	if err != nil {
		return err
	}
	path := filepath.Join(root, ".bws", "config.jsonc")
	var existing *config.Config
	if before != nil {
		if existing, err = config.LoadLocalFile(path); err != nil {
			return err
		}
	}
	registry, err := profile.LoadRegistry(root)
	if err != nil {
		return err
	}
	s := newSetupState(registry, existing, detectedProfiles(root, registry), loadSetupMemory())
	if len(opts.With) > 0 {
		if err := s.selectOnly(opts.With); err != nil {
			return err
		}
	} else if err := runSetupTUI("bws setup — "+FormatWorkspace(root), s); err != nil {
		return err
	}
	if err := saveSetup(root, path, before, s); err != nil {
		return err
	}
	saveSetupMemory(s)
	printSetupSummary(os.Stderr, s, path)
	return nil
}

func saveSetup(root, path string, before []byte, s *setupState) error {
	names := s.profiles()
	pins, err := profile.PinSelections(names, s.registry)
	if err != nil {
		return err
	}
	sel := config.SetupSelection{Profiles: names, Reviewed: pins, Features: s.features()}
	if before != nil {
		return config.ApplySetup(path, sel)
	}
	data, err := config.SetupJSON(sel)
	if err != nil {
		return err
	}
	return config.AtomicPolicyWrite(path, data, nil)
}

func printSetupSummary(w io.Writer, s *setupState, path string) {
	names := s.profiles()
	if len(names) == 0 {
		names = []string{"none"}
	}
	onOff := func(key string) string {
		if s.isOn(key) {
			return "on"
		}
		return "off"
	}
	fmt.Fprintf(w, "[bws] Saved %s: %s; network %s, push %s, GUI %s\n",
		path, strings.Join(names, ", "), onOff("network"), onOff("push"), onOff("gui"))
	fmt.Fprintf(w, "[bws] Change with 'bws setup'; details with 'bws plan'.\n")
}
