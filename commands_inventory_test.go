package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestInventoryExcludesHiddenCommands asserts that the developer inventory does
// not leak hidden commands (inventory/docs), since tools/check_docs_drift.rb
// treats everything it lists as a documented-surface claim.
func TestInventoryExcludesHiddenCommands(t *testing.T) {
	inv := buildInventory(buildApp())
	for _, cmd := range inv.Commands {
		if cmd.Name == "inventory" || cmd.Name == "docs" {
			t.Errorf("inventory lists hidden command %q", cmd.Name)
		}
	}
}

// TestInventoryCapturesAliasesAndFlags checks that a known command carries its
// aliases and its own long flags, which is what the drift checker consumes.
func TestInventoryCapturesAliasesAndFlags(t *testing.T) {
	inv := buildInventory(buildApp())

	var gitWorkflow *commandEntry
	for i := range inv.Commands {
		if inv.Commands[i].Name == "git-workflow" {
			gitWorkflow = &inv.Commands[i]
		}
	}
	if gitWorkflow == nil {
		t.Fatal("git-workflow missing from inventory")
	}
	if !contains(gitWorkflow.Aliases, "gw") {
		t.Errorf("git-workflow aliases = %v, want to include gw", gitWorkflow.Aliases)
	}

	var list *commandEntry
	for i := range gitWorkflow.Subcommands {
		if gitWorkflow.Subcommands[i].Name == "list" {
			list = &gitWorkflow.Subcommands[i]
		}
	}
	if list == nil {
		t.Fatal("git-workflow list missing from inventory")
	}
	if !contains(list.Flags, "--merged") && !contains(list.Flags, "--unmerged") {
		t.Errorf("git-workflow list flags = %v, want a merge filter", list.Flags)
	}
}

// TestInventoryJSONRoundTrip guards the wire shape consumed by the Ruby checker:
// the document must marshal and unmarshal without error and expose global flags.
func TestInventoryJSONRoundTrip(t *testing.T) {
	raw, err := json.Marshal(buildInventory(buildApp()))
	if err != nil {
		t.Fatalf("marshal inventory: %v", err)
	}
	if !strings.Contains(string(raw), "\"global_flags\"") {
		t.Error("inventory JSON has no global_flags key")
	}

	var back commandInventory
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshal inventory: %v", err)
	}
	if len(back.Globals) == 0 {
		t.Error("inventory round-trip lost global flags")
	}
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
