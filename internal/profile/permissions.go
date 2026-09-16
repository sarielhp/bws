package profile

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// PermissionEntries records grants for later review without environment values.
func PermissionEntries(r *ResolvedProfile) []string {
	entries := []string{}
	for _, b := range r.BindsRW {
		entries = append(entries, "writable: "+strings.Join(b, " -> "))
	}
	for _, b := range r.BindsRO {
		entries = append(entries, "read-only: "+strings.Join(b, " -> "))
	}
	for _, v := range r.Mask {
		entries = append(entries, "mask: "+v)
	}
	for _, v := range r.Copy {
		entries = append(entries, "copy: "+v)
	}
	for _, v := range r.PassEnv {
		entries = append(entries, "pass environment: "+v)
	}
	for k := range r.Env {
		entries = append(entries, "environment (value hidden): "+k)
	}
	for _, v := range r.Path {
		entries = append(entries, "path: "+v)
	}
	for k, v := range featureValues(r.Features) {
		entries = append(entries, "feature "+k+": "+v)
	}
	if r.UnshareNet {
		entries = append(entries, "network: isolated")
	}
	sort.Strings(entries)
	return entries
}

// PermissionDigest detects host-rule changes without storing environment values.
func PermissionDigest(r *ResolvedProfile) (string, error) {
	values := map[string]any{
		"path": r.Path, "env": r.Env, "pass_env": r.PassEnv, "mask": r.Mask, "copy": r.Copy,
		"rw": r.BindsRW, "ro": r.BindsRO, "features": r.Features, "offline": r.UnshareNet,
	}
	data, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}
