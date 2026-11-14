package config

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/tailscale/hujson"
)

// KnownKeyAliases maps shorthand names to their nested paths.
var KnownKeyAliases = map[string]string{
	"no_file_limit":        "no_file_limit",
	"disable_file_limit":   "no_file_limit",
	"file_limit":           "max_file_count",
	"max_files":            "max_file_count",
	"max_file_count":       "max_file_count",
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

// NormalizeKey expands shorthand aliases to full dotted paths. Config keys are
// matched case-insensitively, but names inside "env" keep their case because
// environment variables are case-sensitive.
func NormalizeKey(key string) string {
	trimmed := strings.TrimSpace(key)
	lower := strings.ToLower(trimmed)
	if mapped, ok := KnownKeyAliases[lower]; ok {
		return mapped
	}
	if head, rest, ok := strings.Cut(trimmed, "."); ok && strings.ToLower(head) == "env" {
		return "env." + rest
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
				po, ok := obj.Members[i].Value.Value.(*hujson.Object)
				if !ok {
					return fmt.Errorf("key %q is not an object", parentKey)
				}
				parentObj = po
				break
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

	parts := strings.Split(NormalizeKey(key), ".")
	if len(parts) > 1 {
		obj = childObject(obj, parts[0])
	}
	m := findMember(obj, parts[len(parts)-1])
	if m == nil {
		return "", fmt.Errorf("key %q not found", key)
	}
	return renderValue(m.Value), nil
}

// findMember returns obj's member called name, or nil. A nil obj has none.
func findMember(obj *hujson.Object, name string) *hujson.ObjectMember {
	if obj == nil {
		return nil
	}
	for i := range obj.Members {
		if string(obj.Members[i].Name.Value.(hujson.Literal)) == `"`+name+`"` {
			return &obj.Members[i]
		}
	}
	return nil
}

// childObject returns the object stored under name in obj, or nil when
// there is none or it is not an object.
func childObject(obj *hujson.Object, name string) *hujson.Object {
	m := findMember(obj, name)
	if m == nil {
		return nil
	}
	child, _ := m.Value.Value.(*hujson.Object)
	return child
}

// removeMember deletes obj's member called name and reports whether it existed.
func removeMember(obj *hujson.Object, name string) bool {
	before := len(obj.Members)
	obj.Members = slices.DeleteFunc(obj.Members, func(m hujson.ObjectMember) bool {
		return string(m.Name.Value.(hujson.Literal)) == `"`+name+`"`
	})
	return len(obj.Members) != before
}

// UnsetConfigKV removes a key from a JSONC file.
func UnsetConfigKV(path, key string) error {
	parts := strings.Split(NormalizeKey(key), ".")
	return EditJSONC(path, func(root *hujson.Value) error {
		obj, ok := root.Value.(*hujson.Object)
		if !ok {
			return fmt.Errorf("root is not an object")
		}
		if len(parts) == 1 {
			if !removeMember(obj, parts[0]) {
				return fmt.Errorf("key %q not found", key)
			}
			return nil
		}
		parent := childObject(obj, parts[0])
		if parent == nil {
			return fmt.Errorf("key %q not found", key)
		}
		if !removeMember(parent, parts[1]) {
			return fmt.Errorf("key %q not found in %q", parts[1], parts[0])
		}
		return nil
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

// renderValue formats a JSONC value for display: strings are unquoted and
// surrounding whitespace and comments are dropped.
func renderValue(v hujson.Value) string {
	if lit, ok := v.Value.(hujson.Literal); ok && lit.Kind() == '"' {
		return lit.String()
	}
	v.BeforeExtra, v.AfterExtra = nil, nil
	return string(v.Pack())
}
