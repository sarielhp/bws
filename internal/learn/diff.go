package learn

import (
	"bws/internal/config"
	"sort"
	"strings"
)

// IsSubpathOrEqual checks if child is identical to or a subpath of parent.
func IsSubpathOrEqual(child, parent, homeDir string) bool {
	c := CanonicalPath(child, homeDir)
	p := CanonicalPath(parent, homeDir)
	if c == "" || p == "" {
		return false
	}
	return c == p || strings.HasPrefix(c, p+"/")
}

// ComputeDelta calculates newly discovered additions and upgrades compared to an existing configuration.
func ComputeDelta(res *TraceResult, targetConfig *config.Config, homeDir string) *Delta {
	delta := &Delta{
		SecurityAlerts: append([]string{}, res.SecurityAlerts...),
	}

	var existingRW []string
	var existingRO []string
	var existingPaths []string
	var existingFeatures config.FeaturesConfig

	if targetConfig != nil {
		for _, b := range targetConfig.BindsRW {
			if b.Host != "" {
				existingRW = append(existingRW, b.Host)
			}
		}
		for _, b := range targetConfig.BindsRO {
			if b.Host != "" {
				existingRO = append(existingRO, b.Host)
			}
		}
		existingPaths = targetConfig.Path
		if targetConfig.Features != nil {
			existingFeatures = *targetConfig.Features
		}
	}

	// 1. Binary PATH diffing
	if res.DiscoveredPath != "" && !IsPathCovered(res.DiscoveredPath, existingPaths, homeDir) {
		delta.Path = append(delta.Path, res.DiscoveredPath)
	}

	// 2. RW mounts diffing & RO -> RW upgrade detection
	upgradedMap := make(map[string]bool)
	for _, discoveredRW := range res.BindsRW {
		covered := false
		for _, exRW := range existingRW {
			if IsSubpathOrEqual(discoveredRW, exRW, homeDir) {
				covered = true
				break
			}
		}
		if covered {
			continue
		}

		// Check for RO -> RW upgrades
		for _, exRO := range existingRO {
			if IsSubpathOrEqual(exRO, discoveredRW, homeDir) {
				upgradedMap[exRO] = true
			}
		}

		delta.BindsRW = append(delta.BindsRW, discoveredRW)
	}

	for u := range upgradedMap {
		delta.UpgradedRO = append(delta.UpgradedRO, u)
	}
	sort.Strings(delta.UpgradedRO)

	// 3. RO mounts diffing
	for _, discoveredRO := range res.BindsRO {
		covered := false
		// Skip if already in existing RW
		for _, exRW := range existingRW {
			if IsSubpathOrEqual(discoveredRO, exRW, homeDir) {
				covered = true
				break
			}
		}
		if covered {
			continue
		}

		// Skip if covered by newly added RW
		for _, dRW := range delta.BindsRW {
			if IsSubpathOrEqual(discoveredRO, dRW, homeDir) {
				covered = true
				break
			}
		}
		if covered {
			continue
		}

		// Skip if in existing RO (and not being upgraded)
		for _, exRO := range existingRO {
			if !upgradedMap[exRO] && IsSubpathOrEqual(discoveredRO, exRO, homeDir) {
				covered = true
				break
			}
		}
		if covered {
			continue
		}

		delta.BindsRO = append(delta.BindsRO, discoveredRO)
	}

	// 4. Features diffing
	if res.Features.SSH && (existingFeatures.EnableSSH == nil || !*existingFeatures.EnableSSH) {
		delta.Features.SSH = true
	}
	if res.Features.DBus && (existingFeatures.EnableDBus == nil || !*existingFeatures.EnableDBus) {
		delta.Features.DBus = true
	}
	if res.Features.X11 && (existingFeatures.EnableX11 == nil || !*existingFeatures.EnableX11) {
		delta.Features.X11 = true
	}
	if res.Features.WSL && (existingFeatures.EnableWSL == nil || !*existingFeatures.EnableWSL) {
		delta.Features.WSL = true
	}

	return delta
}
