package config

import (
	"maps"
	"slices"
)

// Clone returns a deep copy: no slice, map, or pointer field is shared with c.
func Clone(c *Config) *Config {
	if c == nil {
		return nil
	}
	r := *c
	r.Env = maps.Clone(c.Env)
	r.ReviewedProfiles = maps.Clone(c.ReviewedProfiles)
	r.Profiles = slices.Clone(c.Profiles)
	r.Path = slices.Clone(c.Path)
	r.PassEnv = slices.Clone(c.PassEnv)
	r.Mask = slices.Clone(c.Mask)
	r.Copy = slices.Clone(c.Copy)
	r.BindsRW = slices.Clone(c.BindsRW)
	r.BindsRO = slices.Clone(c.BindsRO)
	r.RejectedBinds = slices.Clone(c.RejectedBinds)
	r.ReviewedStack = clonePtr(c.ReviewedStack)
	r.MaxFileCount = clonePtr(c.MaxFileCount)
	if c.System != nil {
		s := *c.System
		s.ShareNet = clonePtr(s.ShareNet)
		s.Clearenv = clonePtr(s.Clearenv)
		s.UnshareUTS = clonePtr(s.UnshareUTS)
		s.Hostname = clonePtr(s.Hostname)
		s.NewSession = clonePtr(s.NewSession)
		r.System = &s
	}
	r.Features = CloneFeatures(c.Features)
	return &r
}

// CloneFeatures returns a deep copy of f.
func CloneFeatures(f *FeaturesConfig) *FeaturesConfig {
	if f == nil {
		return nil
	}
	v := *f
	v.SSHKeys = slices.Clone(f.SSHKeys)
	v.DBusTalk = slices.Clone(f.DBusTalk)
	for _, p := range []**bool{
		&v.EnableSSH, &v.AutoRepoDeployKey, &v.EnableX11, &v.EnableDBus,
		&v.AllowRawDBus, &v.EnableWSL, &v.EnableEtcAutoBind, &v.EnableProxy,
		&v.NoNet, &v.UnshareNet, &v.MaskHistory, &v.BlockGH,
	} {
		*p = clonePtr(*p)
	}
	return &v
}

func clonePtr[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
