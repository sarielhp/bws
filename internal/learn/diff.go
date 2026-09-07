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

	existingRW, existingRO, existingPaths, existingFeatures := extractExistingConfig(targetConfig)

	// 1. Binary PATH diffing
	delta.Path = diffPaths(res, existingPaths, homeDir)

	// 2. RW mounts diffing & RO -> RW upgrade detection
	deltaRW, upgradedMap := diffBindsRW(res.BindsRW, existingRW, existingRO, homeDir)
	delta.BindsRW = deltaRW

	for u := range upgradedMap {
		delta.UpgradedRO = append(delta.UpgradedRO, u)
	}
	sort.Strings(delta.UpgradedRO)

	// 3. RO mounts diffing
	delta.BindsRO = diffBindsRO(res.BindsRO, existingRW, existingRO, delta.BindsRW, upgradedMap, homeDir)

	// 4. Features diffing
	delta.Features = diffFeatures(res.Features, existingFeatures)

	return delta
}

func extractExistingConfig(targetConfig *config.Config) ([]string, []string, []string, config.FeaturesConfig) {
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

	return existingRW, existingRO, existingPaths, existingFeatures
}

func diffPaths(res *TraceResult, existingPaths []string, homeDir string) []string {
	var candidates []string
	if len(res.DiscoveredPaths) > 0 {
		candidates = append(candidates, res.DiscoveredPaths...)
	}
	if res.DiscoveredPath != "" && !containsPath(candidates, res.DiscoveredPath) {
		candidates = append(candidates, res.DiscoveredPath)
	}

	var deltaPath []string
	for _, p := range candidates {
		if p == "" {
			continue
		}
		if !IsPathCovered(p, existingPaths, homeDir) && !IsPathInList(p, deltaPath, homeDir) {
			deltaPath = append(deltaPath, p)
		}
	}
	return deltaPath
}

func containsPath(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}

func diffBindsRW(discoveredRW, existingRW, existingRO []string, homeDir string) ([]string, map[string]bool) {
	var deltaRW []string
	upgradedMap := make(map[string]bool)

	for _, dRW := range discoveredRW {
		covered := false
		for _, exRW := range existingRW {
			if IsSubpathOrEqual(dRW, exRW, homeDir) {
				covered = true
				break
			}
		}
		if covered {
			continue
		}

		for _, exRO := range existingRO {
			if IsSubpathOrEqual(exRO, dRW, homeDir) {
				upgradedMap[exRO] = true
			}
		}

		deltaRW = append(deltaRW, dRW)
	}

	return deltaRW, upgradedMap
}

func diffBindsRO(discoveredRO, existingRW, existingRO, deltaRW []string, upgradedMap map[string]bool, homeDir string) []string {
	var deltaRO []string

	for _, dRO := range discoveredRO {
		if isCoveredByAny(dRO, existingRW, homeDir) {
			continue
		}
		if isCoveredByAny(dRO, deltaRW, homeDir) {
			continue
		}

		coveredByExistingRO := false
		for _, exRO := range existingRO {
			if !upgradedMap[exRO] && IsSubpathOrEqual(dRO, exRO, homeDir) {
				coveredByExistingRO = true
				break
			}
		}
		if coveredByExistingRO {
			continue
		}

		deltaRO = append(deltaRO, dRO)
	}

	return deltaRO
}

func isCoveredByAny(target string, paths []string, homeDir string) bool {
	for _, p := range paths {
		if IsSubpathOrEqual(target, p, homeDir) {
			return true
		}
	}
	return false
}

func diffFeatures(detected DetectedFeatures, existing config.FeaturesConfig) DetectedFeatures {
	var deltaFeatures DetectedFeatures
	if detected.SSH && (existing.EnableSSH == nil || !*existing.EnableSSH) {
		deltaFeatures.SSH = true
	}
	if detected.DBus && (existing.EnableDBus == nil || !*existing.EnableDBus) {
		deltaFeatures.DBus = true
	}
	if detected.X11 && (existing.EnableX11 == nil || !*existing.EnableX11) {
		deltaFeatures.X11 = true
	}
	if detected.WSL && (existing.EnableWSL == nil || !*existing.EnableWSL) {
		deltaFeatures.WSL = true
	}
	return deltaFeatures
}
