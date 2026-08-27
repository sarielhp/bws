package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/tailscale/hujson"
)

func EditJSONC(path string, fn func(root *hujson.Value) error) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	ast, err := hujson.Parse(data)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}
	if err := fn(&ast); err != nil {
		return err
	}
	out := ast.Pack()
	return os.WriteFile(path, out, 0644)
}

func SetArrayValue(path, key string, newElements []string) error {
	return EditJSONC(path, func(root *hujson.Value) error {
		obj, ok := root.Value.(*hujson.Object)
		if !ok {
			return fmt.Errorf("root is not an object")
		}
		for i := range obj.Members {
			if string(obj.Members[i].Name.Value.(hujson.Literal)) == `"`+key+`"` {
				arr := &hujson.Array{
					Elements: make([]hujson.ArrayElement, len(newElements)),
				}
				for j, e := range newElements {
					arr.Elements[j] = hujson.Value{Value: hujson.String(e)}
				}
				obj.Members[i].Value = hujson.Value{Value: arr}
				return nil
			}
		}
		// Key not found; append a new member with the array value.
		arr := &hujson.Array{
			Elements: make([]hujson.ArrayElement, len(newElements)),
		}
		for j, e := range newElements {
			arr.Elements[j] = hujson.Value{Value: hujson.String(e)}
		}
		obj.Members = append(obj.Members, hujson.ObjectMember{
			Name:  hujson.Value{Value: hujson.String(key)},
			Value: hujson.Value{Value: arr},
		})
		return nil
	})
}

func AddArrayElement(path, key, element string) error {
	return EditJSONC(path, func(root *hujson.Value) error {
		obj, ok := root.Value.(*hujson.Object)
		if !ok {
			return fmt.Errorf("root is not an object")
		}
		for i := range obj.Members {
			if string(obj.Members[i].Name.Value.(hujson.Literal)) == `"`+key+`"` {
				arr, ok := obj.Members[i].Value.Value.(*hujson.Array)
				if !ok {
					return fmt.Errorf("key %q is not an array", key)
				}
				elem := hujson.Value{Value: hujson.String(element)}
				arr.Elements = append(arr.Elements, elem)
				return nil
			}
		}
		// Key not found; create it with the element as initial array.
		arr := &hujson.Array{
			Elements: []hujson.ArrayElement{{Value: hujson.String(element)}},
		}
		obj.Members = append(obj.Members, hujson.ObjectMember{
			Name:  hujson.Value{Value: hujson.String(key)},
			Value: hujson.Value{Value: arr},
		})
		return nil
	})
}

func AddBindArrayElement(path, key, entry string) error {
	return EditJSONC(path, func(root *hujson.Value) error {
		obj, ok := root.Value.(*hujson.Object)
		if !ok {
			return fmt.Errorf("root is not an object")
		}
		for i := range obj.Members {
			if string(obj.Members[i].Name.Value.(hujson.Literal)) == `"`+key+`"` {
				arr, ok := obj.Members[i].Value.Value.(*hujson.Array)
				if !ok {
					return fmt.Errorf("key %q is not an array", key)
				}
				elem, err := hujson.Parse([]byte(entry))
				if err != nil {
					return fmt.Errorf("parsing bind entry: %w", err)
				}
				arr.Elements = append(arr.Elements, elem)
				return nil
			}
		}
		// Key not found; create it with the entry as initial array element.
		elem, err := hujson.Parse([]byte(entry))
		if err != nil {
			return fmt.Errorf("parsing bind entry: %w", err)
		}
		arr := &hujson.Array{Elements: []hujson.ArrayElement{elem}}
		obj.Members = append(obj.Members, hujson.ObjectMember{
			Name:  hujson.Value{Value: hujson.String(key)},
			Value: hujson.Value{Value: arr},
		})
		return nil
	})
}

func RemoveArrayElement(path, key, match string) error {
	return EditJSONC(path, func(root *hujson.Value) error {
		obj, ok := root.Value.(*hujson.Object)
		if !ok {
			return fmt.Errorf("root is not an object")
		}
		for i := range obj.Members {
			if string(obj.Members[i].Name.Value.(hujson.Literal)) == `"`+key+`"` {
				arr, ok := obj.Members[i].Value.Value.(*hujson.Array)
				if !ok {
					return fmt.Errorf("key %q is not an array", key)
				}
				filtered := make([]hujson.ArrayElement, 0, len(arr.Elements))
				for _, e := range arr.Elements {
					packed := string(e.Pack())
					if strings.Contains(packed, `"`+match+`"`) {
						continue
					}
					filtered = append(filtered, e)
				}
				if len(filtered) == len(arr.Elements) {
					return fmt.Errorf("entry %q not found in array %q", match, key)
				}
				arr.Elements = filtered
				return nil
			}
		}
		return fmt.Errorf("key %q not found in root object", key)
	})
}

func RemoveBindElement(path, key, hostPath string) (bool, error) {
	found := false
	err := EditJSONC(path, func(root *hujson.Value) error {
		obj, ok := root.Value.(*hujson.Object)
		if !ok {
			return fmt.Errorf("root is not an object")
		}
		for i := range obj.Members {
			if string(obj.Members[i].Name.Value.(hujson.Literal)) == `"`+key+`"` {
				arr, ok := obj.Members[i].Value.Value.(*hujson.Array)
				if !ok {
					return fmt.Errorf("key %q is not an array", key)
				}
				filtered := make([]hujson.ArrayElement, 0, len(arr.Elements))
				for _, e := range arr.Elements {
					packed := string(e.Pack())
					if strings.Contains(packed, `"`+hostPath+`"`) {
						found = true
						continue
					}
					filtered = append(filtered, e)
				}
				arr.Elements = filtered
				return nil
			}
		}
		return fmt.Errorf("key %q not found in root object", key)
	})
	return found, err
}

func LoadFileWithHuJSON(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	standardized, err := hujson.Standardize(data)
	if err != nil {
		return nil, fmt.Errorf("invalid JSON/JSONC in %s: %w", path, err)
	}
	return standardized, nil
}
