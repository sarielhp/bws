package main

import (
	"strings"
	"testing"

	"github.com/sarielhp/clihelp"
	"github.com/sarielhp/clihelp/clihelptest"
)

// libraryCommandName reports whether name is a command supplied by clihelp
// itself (mounted via the library constructors) rather than authored by bws.
func libraryCommandName(name string) bool {
	switch name {
	case "manpage", "completion":
		return true
	}
	return false
}

// TestCommandTreeWalkInvariants uses clihelp's Walk to assert that every command
// in the tree is well-formed: it is either a runnable leaf or a group node, it
// carries a short description that stays on one row, and a grouped command has
// a Group label.
func TestCommandTreeWalkInvariants(t *testing.T) {
	app := buildApp()
	err := app.Walk(func(path []string, cmd *clihelp.Command) error {
		if cmd.Hidden {
			return nil
		}
		joined := strings.Join(path, " ")

		if cmd.Run == nil && len(cmd.Subcommands) == 0 {
			t.Errorf("%s: command has neither a Run handler nor subcommands", joined)
		}
		if strings.TrimSpace(cmd.Description) == "" {
			t.Errorf("%s: command has no short description", joined)
		}
		if strings.Contains(cmd.Description, "\n") {
			t.Errorf("%s: description must be a single line", joined)
		}
		// Library-supplied commands (manpage, completion) carry no Group and
		// are not bws's to label.
		if cmd.Group == "" && len(path) == 1 && !libraryCommandName(cmd.Name) {
			t.Errorf("%s: top-level command has no Group label", joined)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Walk returned an error: %v", err)
	}
}

// TestLookupCommandReachable spot-checks that the mounted library commands and
// primary commands resolve through LookupCommand.
func TestLookupCommandReachable(t *testing.T) {
	app := buildApp()
	for _, path := range [][]string{
		{"init"},
		{"mount", "add"},
		{"profile", "save"},
		{"config", "completion"},
		{"manpage"},
	} {
		if app.LookupCommand(path...) == nil {
			t.Errorf("expected command %q to resolve", strings.Join(path, " "))
		}
	}
}

// TestNativeExamplesFlag verifies the built-in -E/--examples topic renders.
func TestNativeExamplesFlag(t *testing.T) {
	res := clihelptest.Execute(buildApp(), []string{"-E"})
	res.AssertNoError(t)
	res.AssertStdoutContains(t, "Examples:")
}

// TestManPageCommand verifies the mounted ManPageCommand prints the manual,
// and that its namespaced flags let it coexist with bws's persistent --force.
func TestManPageCommand(t *testing.T) {
	res := clihelptest.Execute(buildApp(), []string{"manpage"})
	res.AssertNoError(t)
	res.AssertStdoutContains(t, ".TH \"BWS\"")
	res.AssertStdoutContains(t, "bws \\- ")
}

// TestMountManPageCommandWithForceGlobal is the regression guard for
// clihelp#1: the app's global --force must not collide with the library
// command's now-namespaced --man-force.
func TestMountManPageCommandWithForceGlobal(t *testing.T) {
	app := buildApp()
	if app.LookupCommand("manpage") == nil {
		t.Fatal("expected manpage command to be mounted")
	}
	if err := clihelp.Audit(app); err != nil {
		t.Fatalf("Audit rejected the app with ManPageCommand mounted: %v", err)
	}
}

// TestDocsManIsHidden ensures the docs command does not clutter help output.
func TestDocsManIsHidden(t *testing.T) {
	res := clihelptest.Execute(buildApp(), []string{"--help"})
	res.AssertNoError(t)
	if strings.Contains(res.Stdout, "\n  docs") {
		t.Errorf("hidden docs command should not appear in help output")
	}
}
