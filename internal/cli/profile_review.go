package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	"bws/internal/config"
	"bws/internal/policy"
	"bws/internal/profile"
)

func printCompoundDetails(p *profile.Profile, registry map[string]*profile.Profile) error {
	if p.Kind != "compound" {
		return nil
	}
	fmt.Printf("Kind: compound\nDefinition: %s\n", p.Origin)
	if p.Detect != nil {
		if err := writeJSON(os.Stdout, p.Detect); err != nil {
			return err
		}
	}
	keys := make([]string, 0, len(p.Reviewed))
	for k := range p.Reviewed {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("Reviewed dependency: %s [%s] %s\n", k, p.Reviewed[k].Source, p.Reviewed[k].SHA256)
	}
	cfg := &config.Config{Profiles: []string{p.Name}}
	if err := policy.ApplyRegistry(cfg, registry); err != nil {
		return err
	}
	fmt.Println("Profile contribution (global/project configuration may add access):")
	PrintPolicySummary(os.Stdout, cfg)
	return nil
}

// HandleProfileReview refreshes dependency fingerprints only on explicit approval.
func HandleProfileReview(name string, accept bool) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	registry, err := profile.LoadRegistry(cwd)
	if err != nil {
		return err
	}
	original, ok := registry[name]
	if !ok {
		return fmt.Errorf("unknown profile %q", name)
	}
	if original.Origin == "" {
		return fmt.Errorf("embedded profiles cannot be updated; compose a named profile first")
	}
	if original.Kind != "compound" {
		return fmt.Errorf("dependency review applies to saved compound profiles; inspect this tool with 'bws profile show %s'", name)
	}
	before, err := config.PolicyBytes(original.Origin)
	if err != nil {
		return err
	}
	p := *original
	if err := profile.Pin(&p, registry); err != nil {
		return err
	}
	var changes []string
	for k, v := range p.Reviewed {
		if old, ok := original.Reviewed[k]; !ok || old != v {
			changes = append(changes, k)
		}
	}
	for k := range original.Reviewed {
		if _, ok := p.Reviewed[k]; !ok {
			changes = append(changes, k+" (removed)")
		}
	}
	sort.Strings(changes)
	fmt.Printf("Dependency changes for %s: %s\n", name, strings.Join(changes, ", "))
	printPermissionChanges(original.ReviewedPermissions, p.ReviewedPermissions)
	registry[name] = &p
	registry[p.Name] = &p
	for _, alias := range p.Aliases {
		registry[alias] = &p
	}
	if err := printCompoundDetails(&p, registry); err != nil {
		return err
	}
	if !accept {
		fmt.Println("No changes written. Review the grants, then rerun with --accept.")
		return nil
	}
	data, err := json.MarshalIndent(&p, "", "  ")
	if err != nil {
		return err
	}
	if err := config.AtomicPolicyWrite(original.Origin, append(data, '\n'), before); err != nil {
		return err
	}
	fmt.Println("Updated dependency approval.")
	return nil
}

func printPermissionChanges(before, after []string) {
	if before == nil {
		fmt.Println("No previous permission baseline; review all current grants.")
	}
	for _, v := range after {
		if !slices.Contains(before, v) {
			fmt.Printf("  + %s\n", v)
		}
	}
	for _, v := range before {
		if !slices.Contains(after, v) {
			fmt.Printf("  - %s\n", v)
		}
	}
	fmt.Println("Environment values are hidden; dependency fingerprints also track changes to values and other metadata.")
}
