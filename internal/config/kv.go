package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/tailscale/hujson"
)

// KnownKeyAliases maps shorthand names to their nested paths.
var KnownKeyAliases = map[string]string{
	"enable_proxy":         "features.enable_proxy",
	"enable_ssh":           "features.enable_ssh",
	"enable_x11":           "features.enable_x11",
	"enable_dbus":          "features.enable_dbus",
	"allow_raw_dbus":       "features.allow_raw_dbus",
	"dbus_talk":            "features.dbus_talk",
	"enable_wsl":           "features.enable_wsl",
	"enable_etc_auto_bind": "features.enable_etc_auto_bind",
	"auto_repo_deploy_key": "features.auto_repo_deploy_key",
	"no_net":               "features.no_net",
	"unshare_net":          "features.unshare_net",
	"share_net":            "system.share_net",
	"clearenv":             "system.clearenv",
	"unshare_uts":          "system.unshare_uts",
	"hostname":             "system.hostname",
}

// NormalizeKey expands shorthand aliases to full dotted paths.
func NormalizeKey(key string) string {
	lower := strings.ToLower(strings.TrimSpace(key))
	if mapped, ok := KnownKeyAliases[lower]; ok {
		return mapped
	}
	return lower
}

// SetConfigKV sets a scalar or boolean key value in a JSONC file.
func SetConfigKV(path, key, rawValue string) error {
	normalized := NormalizeKey(key)
	parts := strings.Split(normalized, ".")

	return EditJSONC(path, func(root *hujson.Value) error {
		obj, ok := root.Value.(*hujson.Object)
		if !ok {
			return fmt.Errorf("root is not an object")
		}

		valNode := parseValueToHuJSON(rawValue)

		if len(parts) == 1 {
			setInObject(obj, parts[0], valNode)
			return nil
		}

		// Nested: e.g. features.enable_proxy
		parentKey := parts[0]
		childKey := parts[1]

		var parentObj *hujson.Object
		for i := range obj.Members {
			if string(obj.Members[i].Name.Value.(hujson.Literal)) == `"`+parentKey+`"` {
				if po, ok := obj.Members[i].Value.Value.(*hujson.Object); ok {
					parentObj = po
					break
				}
			}
		}

		if parentObj == nil {
			parentObj = &hujson.Object{Members: []hujson.ObjectMember{}}
			obj.Members = append(obj.Members, hujson.ObjectMember{
				Name:  hujson.Value{Value: hujson.String(parentKey)},
				Value: hujson.Value{Value: parentObj},
			})
		}

		setInObject(parentObj, childKey, valNode)
		return nil
	})
}

// GetConfigKV retrieves a string representation of a key value from a JSONC file.
func GetConfigKV(path, key string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	ast, err := hujson.Parse(data)
	if err != nil {
		return "", fmt.Errorf("parsing %s: %w", path, err)
	}

	obj, ok := ast.Value.(*hujson.Object)
	if !ok {
		return "", fmt.Errorf("root is not an object")
	}

	normalized := NormalizeKey(key)
	parts := strings.Split(normalized, ".")

	if len(parts) == 1 {
		for _, m := range obj.Members {
			if string(m.Name.Value.(hujson.Literal)) == `"`+parts[0]+`"` {
				return string(m.Value.Pack()), nil
			}
		}
		return "", fmt.Errorf("key %q not found", key)
	}

	parentKey := parts[0]
	childKey := parts[1]
	for _, m := range obj.Members {
		if string(m.Name.Value.(hujson.Literal)) == `"`+parentKey+`"` {
			if po, ok := m.Value.Value.(*hujson.Object); ok {
				for _, cm := range po.Members {
					if string(cm.Name.Value.(hujson.Literal)) == `"`+childKey+`"` {
						return string(cm.Value.Pack()), nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("key %q not found", key)
}

// UnsetConfigKV removes a key from a JSONC file.
func UnsetConfigKV(path, key string) error {
	normalized := NormalizeKey(key)
	parts := strings.Split(normalized, ".")

	return EditJSONC(path, func(root *hujson.Value) error {
		obj, ok := root.Value.(*hujson.Object)
		if !ok {
			return fmt.Errorf("root is not an object")
		}

		if len(parts) == 1 {
			filtered := make([]hujson.ObjectMember, 0, len(obj.Members))
			found := false
			for _, m := range obj.Members {
				if string(m.Name.Value.(hujson.Literal)) == `"`+parts[0]+`"` {
					found = true
					continue
				}
				filtered = append(filtered, m)
			}
			if !found {
				return fmt.Errorf("key %q not found", key)
			}
			obj.Members = filtered
			return nil
		}

		parentKey := parts[0]
		childKey := parts[1]
		for i := range obj.Members {
			if string(obj.Members[i].Name.Value.(hujson.Literal)) == `"`+parentKey+`"` {
				if po, ok := obj.Members[i].Value.Value.(*hujson.Object); ok {
					filtered := make([]hujson.ObjectMember, 0, len(po.Members))
					found := false
					for _, cm := range po.Members {
						if string(cm.Name.Value.(hujson.Literal)) == `"`+childKey+`"` {
							found = true
							continue
						}
						filtered = append(filtered, cm)
					}
					if !found {
						return fmt.Errorf("key %q not found in %q", childKey, parentKey)
					}
					po.Members = filtered
					return nil
				}
			}
		}

		return fmt.Errorf("key %q not found", key)
	})
}

func parseValueToHuJSON(rawValue string) hujson.Value {
	trimmed := strings.TrimSpace(rawValue)
	if trimmed == "true" || trimmed == "false" {
		return hujson.Value{Value: hujson.Bool(trimmed == "true")}
	}
	if num, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		return hujson.Value{Value: hujson.Literal(strconv.FormatInt(num, 10))}
	}
	return hujson.Value{Value: hujson.String(trimmed)}
}

func setInObject(obj *hujson.Object, key string, valNode hujson.Value) {
	for i := range obj.Members {
		if string(obj.Members[i].Name.Value.(hujson.Literal)) == `"`+key+`"` {
			obj.Members[i].Value = valNode
			return
		}
	}
	obj.Members = append(obj.Members, hujson.ObjectMember{
		Name:  hujson.Value{Value: hujson.String(key)},
		Value: valNode,
	})
}
