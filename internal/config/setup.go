package config

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/tailscale/hujson"
)

// SetupSelection is the workspace policy chosen on the setup screen.
type SetupSelection struct {
	Profiles []string
	Reviewed map[string]ProfileApproval
	Features map[string]bool // keys under "features", e.g. "no_net"
}

// SetupJSON renders a selection as a new local configuration file.
func SetupJSON(sel SetupSelection) ([]byte, error) {
	data, err := json.MarshalIndent(struct {
		Profiles []string                   `json:"profiles"`
		Reviewed map[string]ProfileApproval `json:"reviewed_profiles,omitempty"`
		Features map[string]bool            `json:"features,omitempty"`
	}{sel.Profiles, sel.Reviewed, sel.Features}, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// ApplySetup writes a selection into an existing, trusted local configuration
// in a single edit. Settings the setup screen does not manage (mounts, env,
// stack, comments) are kept.
func ApplySetup(path string, sel SetupSelection) error {
	profiles, err := jsonValue(sel.Profiles)
	if err != nil {
		return err
	}
	reviewed, err := jsonValue(sel.Reviewed)
	if err != nil {
		return err
	}
	return EditJSONC(path, func(root *hujson.Value) error {
		obj, ok := root.Value.(*hujson.Object)
		if !ok {
			return fmt.Errorf("%s: root is not an object", path)
		}
		setInObject(obj, "profiles", profiles)
		setInObject(obj, "reviewed_profiles", reviewed)
		features, err := childObject(obj, "features")
		if err != nil {
			return err
		}
		keys := make([]string, 0, len(sel.Features))
		for k := range sel.Features {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			setInObject(features, k, hujson.Value{Value: hujson.Bool(sel.Features[k])})
		}
		root.Format()
		return nil
	})
}

func jsonValue(v any) (hujson.Value, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return hujson.Value{}, err
	}
	return hujson.Parse(data)
}

// childObject returns obj[key] as an object, creating it when absent.
func childObject(obj *hujson.Object, key string) (*hujson.Object, error) {
	for i := range obj.Members {
		if string(obj.Members[i].Name.Value.(hujson.Literal)) != `"`+key+`"` {
			continue
		}
		child, ok := obj.Members[i].Value.Value.(*hujson.Object)
		if !ok {
			return nil, fmt.Errorf("key %q is not an object", key)
		}
		return child, nil
	}
	child := &hujson.Object{}
	obj.Members = append(obj.Members, hujson.ObjectMember{
		Name:  hujson.Value{Value: hujson.String(key)},
		Value: hujson.Value{Value: child},
	})
	return child, nil
}
