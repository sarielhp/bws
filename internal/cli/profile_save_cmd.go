package cli

import (
	"bws/internal/config"
	"bws/internal/detect"
	"bws/internal/policy"
	"bws/internal/profile"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// ProfileSaveOptions controls preview and explicit export acknowledgments.
type ProfileSaveOptions struct {
	Name              string
	Description       string
	Local             bool
	Force             bool
	DryRun            bool
	Yes               bool
	Flatten           bool
	AllowMachinePaths bool
	Omit              []string
	Match             []string
	NoDetect          bool
	Profiles          []string
	Flags             policy.Flags
}

// HandleProfileSave exports effective policy or composes explicit dependencies.
func HandleProfileSave(opts ProfileSaveOptions) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	target, err := profile.Target(opts.Name, cwd, opts.Local)
	if err != nil {
		return err
	}
	before, err := config.PolicyBytes(target)
	if err != nil {
		return err
	}
	if before != nil && !opts.Force {
		return fmt.Errorf("profile %q already exists; use --force to overwrite", opts.Name)
	}
	resolution, err := policy.Load(cwd)
	if err != nil {
		return err
	}
	opts.Flags.Apply(resolution.Config)
	registry, err := profile.LoadRegistry(cwd)
	if err != nil {
		return err
	}
	target, before, err = checkedProfileDestination(target, registry, opts)
	if err != nil {
		return err
	}
	plan, err := prepareProfileSave(resolution, registry, opts)
	if err != nil {
		return err
	}
	if !opts.Local {
		for name, dep := range plan.Profile.Reviewed {
			if dep.Source == "local" {
				return fmt.Errorf("global profile would depend on workspace-local %q; use --flatten for portability, or save --local", name)
			}
		}
	}
	if err := configureSaveDetection(plan.Profile, resolution.Workspace, opts); err != nil {
		return err
	}
	printSavePreview(target, resolution, plan, before)
	if len(opts.Profiles) == 0 {
		if err := printSaveOrigins(resolution, registry); err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(plan.Profile, "", "  ")
	if err != nil {
		return err
	}
	if opts.DryRun {
		_, err = os.Stdout.Write(append(data, '\n'))
		return err
	}
	if err := acknowledgeExport(plan, opts); err != nil {
		return err
	}
	if !opts.Yes && IsInteractiveTTY(int(os.Stdin.Fd())) {
		if err := confirmPolicy(os.Stdin, os.Stderr, "Save this profile and its matching rules?"); err != nil {
			return err
		}
	}
	if err := config.AtomicPolicyWrite(target, append(data, '\n'), before); err != nil {
		return err
	}
	fmt.Printf("Saved environment snapshot as profile %q (%s).\nNot activated. Initialize another project with: bws init --profile %s\n", opts.Name, target, opts.Name)
	return nil
}

func printSaveOrigins(r *policy.Resolution, registry map[string]*profile.Profile) error {
	origins, err := policy.Explain(r, registry)
	if err != nil {
		return err
	}
	for _, b := range r.Config.BindsRW {
		if sources := policy.WritableOrigins(origins, b, r.Workspace); len(sources) > 0 {
			fmt.Fprintf(os.Stderr, "  Writable source %s: %s\n", b.Host, strings.Join(sources, "; "))
		}
	}
	return nil
}

func prepareProfileSave(r *policy.Resolution, registry map[string]*profile.Profile, opts ProfileSaveOptions) (*policy.ExportPlan, error) {
	if len(opts.Profiles) == 0 {
		return policy.Export(r, registry, policy.ExportOptions{Name: opts.Name, Description: opts.Description, Flatten: opts.Flatten})
	}
	p := &profile.Profile{Name: opts.Name, Kind: "compound", Description: opts.Description, Requires: opts.Profiles}
	cfg := &config.Config{}
	opts.Flags.Apply(cfg)
	p.Features = cfg.Features
	if err := profile.Pin(p, registry); err != nil {
		return nil, err
	}
	reg := make(map[string]*profile.Profile, len(registry)+1)
	for k, v := range registry {
		reg[k] = v
	}
	reg[p.Name] = p
	if _, err := profile.ResolveProfile(p.Name, reg, profile.DetectMatchContext()); err != nil {
		return nil, err
	}
	effective := config.Merge(r.Global, &config.Config{Profiles: []string{p.Name}})
	if err := policy.ApplyRegistry(effective, reg); err != nil {
		return nil, err
	}
	policy.ExpandWorkspace(effective, r.Workspace)
	r.Config = effective
	if opts.Flatten {
		return policy.Export(r, reg, policy.ExportOptions{Name: opts.Name, Description: opts.Description, Flatten: true})
	}
	return policy.InspectExport(p, effective, r.Workspace)
}

func configureSaveDetection(p *profile.Profile, root string, opts ProfileSaveOptions) error {
	if opts.NoDetect && len(opts.Match) != 0 {
		return fmt.Errorf("--no-detect and --match are mutually exclusive")
	}
	if len(opts.Match) != 0 {
		p.Detect = &profile.DetectSpec{Files: opts.Match}
	}
	if p.Detect == nil && !opts.NoDetect && len(opts.Profiles) == 0 {
		evidence, err := detect.Scan(root)
		if err != nil {
			return err
		}
		p.Detect = profile.SuggestedDetect(evidence)
	}
	return profile.ValidateDetect(p.Detect)
}

func printSavePreview(target string, r *policy.Resolution, plan *policy.ExportPlan, before []byte) {
	fmt.Fprintf(os.Stderr, "Profile destination: %s\nSources: %s + %s\nDependencies: %s\n",
		target, r.GlobalPath, r.LocalPath, strings.Join(plan.Profile.Requires, ", "))
	if before != nil {
		fmt.Fprintln(os.Stderr, "Replacing an existing definition; review the proposed JSON with --dry-run.")
	}
	PrintPolicySummary(os.Stderr, r.Config)
	if plan.Profile.Detect != nil {
		data, err := json.Marshal(plan.Profile.Detect)
		if err == nil {
			fmt.Fprintf(os.Stderr, "Proposed detection rules: %s\n", data)
		}
	} else {
		fmt.Fprintln(os.Stderr, "No automatic matching rules.")
	}
	if len(plan.Unsupported) > 0 {
		fmt.Fprintf(os.Stderr, "Not represented: %s\n", strings.Join(plan.Unsupported, ", "))
	}
	if len(plan.MachinePaths) > 0 {
		fmt.Fprintf(os.Stderr, "Machine-specific paths: %s\n", strings.Join(plan.MachinePaths, ", "))
	}
	fmt.Fprintln(os.Stderr, "This saves capabilities, not installed tools, credentials, or running processes.")
}

func acknowledgeExport(plan *policy.ExportPlan, opts ProfileSaveOptions) error {
	for _, field := range plan.Unsupported {
		if !slices.Contains(opts.Omit, field) {
			return fmt.Errorf("setting %s cannot be represented; acknowledge with --omit %s", field, field)
		}
	}
	for _, field := range opts.Omit {
		if !slices.Contains(plan.Unsupported, field) {
			return fmt.Errorf("--omit %s does not name an omitted setting", field)
		}
	}
	if len(plan.MachinePaths) > 0 && !opts.AllowMachinePaths {
		return fmt.Errorf("machine-specific paths require --allow-machine-paths after review")
	}
	return nil
}

func checkedProfileDestination(target string, registry map[string]*profile.Profile, opts ProfileSaveOptions) (string, []byte, error) {
	existing := registry[opts.Name]
	if existing != nil {
		if existing.Name != opts.Name {
			return "", nil, fmt.Errorf("%q is an alias for %q; choose a distinct name", opts.Name, existing.Name)
		}
		if !opts.Force {
			return "", nil, fmt.Errorf("profile %q already exists or would be shadowed; use --force after review", opts.Name)
		}
		if existing.Origin != "" && filepath.Dir(existing.Origin) == filepath.Dir(target) {
			target = existing.Origin
		}
	}
	before, err := config.PolicyBytes(target)
	return target, before, err
}
