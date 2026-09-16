package config

import (
	"maps"
	"slices"
)

// Clone returns independently mutable configuration collections and structs.
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
	if c.System != nil {
		v := *c.System
		r.System = &v
	}
	if c.Features != nil {
		v := *c.Features
		v.SSHKeys = slices.Clone(c.Features.SSHKeys)
		v.DBusTalk = slices.Clone(c.Features.DBusTalk)
		r.Features = &v
	}
	return &r
}
