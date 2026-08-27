package config

func boolPtrVal(b *bool) bool {
	if b != nil {
		return *b
	}
	return false
}

func Merge(global, local *Config) *Config {
	if local == nil {
		return global
	}
	result := *global

	if local.SandboxPath != "" {
		result.SandboxPath = local.SandboxPath
	}
	if local.ModelsJSONPath != "" {
		result.ModelsJSONPath = local.ModelsJSONPath
	}
	if local.TmuxSessionName != "" {
		result.TmuxSessionName = local.TmuxSessionName
	}
	if local.MaxFileCount != 0 {
		result.MaxFileCount = local.MaxFileCount
	}
	if local.Cdtoday != "" {
		result.Cdtoday = local.Cdtoday
	}

	result.System = mergeSystem(global.System, local.System)
	result.Features = mergeFeatures(global.Features, local.Features)
	result.OhMyPosh = mergeOhMyPosh(global.OhMyPosh, local.OhMyPosh)
	result.Env = mergeEnv(global.Env, local.Env)

	result.Path = mergeStringSlices(global.Path, local.Path)
	result.BindsRW = mergeBindEntries(global.BindsRW, local.BindsRW)
	result.BindsRO = mergeBindEntries(global.BindsRO, local.BindsRO)
	result.Copy = mergeStringSlices(global.Copy, local.Copy)

	return &result
}

func mergeStringSlices(global, local []string) []string {
	if len(local) == 0 {
		return global
	}
	if len(global) == 0 {
		return local
	}
	seen := make(map[string]bool)
	var result []string
	for _, item := range global {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	for _, item := range local {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

func mergeBindEntries(global, local []BindEntry) []BindEntry {
	if len(local) == 0 {
		return global
	}
	if len(global) == 0 {
		return local
	}
	// Index by Host path so local bind settings can update/extend global binds
	seen := make(map[string]int)
	var result []BindEntry
	for _, item := range global {
		seen[item.Host] = len(result)
		result = append(result, item)
	}
	for _, item := range local {
		if idx, exists := seen[item.Host]; exists {
			result[idx] = item
		} else {
			seen[item.Host] = len(result)
			result = append(result, item)
		}
	}
	return result
}

func mergeSystem(global, local *SystemConfig) *SystemConfig {
	if local == nil {
		return global
	}
	if global == nil {
		return local
	}
	r := *global
	if local.ShareNet != nil {
		r.ShareNet = local.ShareNet
	}
	if local.Clearenv != nil {
		r.Clearenv = local.Clearenv
	}
	if local.UnshareUTS != nil {
		r.UnshareUTS = local.UnshareUTS
	}
	if local.Hostname != nil {
		r.Hostname = local.Hostname
	}
	return &r
}

func mergeFeatures(global, local *FeaturesConfig) *FeaturesConfig {
	if local == nil {
		return global
	}
	if global == nil {
		return local
	}
	r := *global
	if local.EnableSSH != nil {
		r.EnableSSH = local.EnableSSH
	}
	if local.SSHKeys != nil {
		r.SSHKeys = local.SSHKeys
	}
	if local.AutoRepoDeployKey != nil {
		r.AutoRepoDeployKey = local.AutoRepoDeployKey
	}
	if local.EnableX11 != nil {
		r.EnableX11 = local.EnableX11
	}
	if local.EnableWSL != nil {
		r.EnableWSL = local.EnableWSL
	}
	if local.EnableEtcAutoBind != nil {
		r.EnableEtcAutoBind = local.EnableEtcAutoBind
	}
	if local.EnableOhMyPosh != nil {
		r.EnableOhMyPosh = local.EnableOhMyPosh
	}
	return &r
}

func mergeOhMyPosh(global, local *OhMyPoshConfig) *OhMyPoshConfig {
	if local == nil {
		return global
	}
	if global == nil {
		return local
	}
	r := *global
	if local.ThemePath != nil {
		r.ThemePath = local.ThemePath
	}
	return &r
}

func mergeEnv(global, local map[string]string) map[string]string {
	if local == nil {
		return global
	}
	if global == nil {
		return local
	}
	r := make(map[string]string, len(global)+len(local))
	for k, v := range global {
		r[k] = v
	}
	for k, v := range local {
		r[k] = v
	}
	return r
}

func GetBool(cfg *Config, getter func(*Config) *bool, defaultVal bool) bool {
	if cfg == nil {
		return defaultVal
	}
	b := getter(cfg)
	if b != nil {
		return *b
	}
	return defaultVal
}

func GetString(cfg *Config, getter func(*Config) *string, defaultVal string) string {
	if cfg == nil {
		return defaultVal
	}
	s := getter(cfg)
	if s != nil {
		return *s
	}
	return defaultVal
}

func FeatureEnabled(cfg *Config, getter func(*FeaturesConfig) *bool) bool {
	if cfg == nil || cfg.Features == nil {
		return false
	}
	b := getter(cfg.Features)
	if b != nil {
		return *b
	}
	return false
}
