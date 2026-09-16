// Package detect collects bounded, read-only evidence about project contents.
package detect

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Entry describes a regular file or directory without reading its contents.
type Entry struct {
	Path      string `json:"path"`
	Directory bool   `json:"directory,omitempty"`
}

// Evidence is a bounded inventory rooted at the selected workspace.
type Evidence struct {
	Root    string  `json:"root"`
	Entries []Entry `json:"entries"`
}

const MaxEntries = 10000
const MaxDepth = 3

// Scan inspects filenames without following symlinks or executing project code.
func Scan(dir string) (*Evidence, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(abs)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	result := &Evidence{Root: abs}
	count := 0
	if err := scanDirectory(root, ".", 0, result, &count); err != nil {
		return nil, err
	}
	sort.Slice(result.Entries, func(i, j int) bool { return result.Entries[i].Path < result.Entries[j].Path })
	return result, nil
}

func scanDirectory(root *os.Root, path string, depth int, out *Evidence, count *int) error {
	f, err := root.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	for {
		entries, err := f.ReadDir(128)
		if err != nil && err != io.EOF {
			return err
		}
		for _, e := range entries {
			*count++
			if *count > MaxEntries {
				return fmt.Errorf("project detection exceeded %d entries; select a profile explicitly", MaxEntries)
			}
			if e.Type()&os.ModeSymlink != 0 {
				continue
			}
			rel := filepath.Join(path, e.Name())
			if !e.IsDir() && !e.Type().IsRegular() {
				continue
			}
			out.Entries = append(out.Entries, Entry{Path: filepath.ToSlash(rel), Directory: e.IsDir()})
			if e.IsDir() && depth < MaxDepth && !skip(e.Name()) {
				if err := scanDirectory(root, rel, depth+1, out, count); err != nil {
					return err
				}
			}
		}
		if err == io.EOF {
			return nil
		}
	}
}

func skip(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	switch name {
	case "node_modules", "vendor", "target", "dist", "build", "__pycache__", "venv":
		return true
	}
	return false
}
