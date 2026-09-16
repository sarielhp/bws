package config

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TrustDir is host-only state recording reviewed local configuration contents.
func TrustDir() string {
	return filepath.Join(ConfigDir(), "trusted")
}

func trustRecord(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	key := sha256.Sum256([]byte(abs))
	return filepath.Join(TrustDir(), fmt.Sprintf("%x", key)), nil
}

// ReadTrustedFile returns the exact bytes approved on the host, or refuses them.
func ReadTrustedFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	record, err := trustRecord(path)
	if err != nil {
		return nil, err
	}
	approved, err := os.ReadFile(record)
	digest := fmt.Sprintf("%x", sha256.Sum256(data))
	if err != nil || string(approved) != digest {
		return nil, fmt.Errorf("untrusted or changed local configuration %s; review it, then run 'bws config trust' in its workspace", path)
	}
	return data, nil
}

// TrustFile records explicit approval of a file's current contents.
func TrustFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return trustContents(path, data)
}

func trustContents(path string, data []byte) error {
	record, err := trustRecord(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(TrustDir(), 0700); err != nil {
		return err
	}
	return os.WriteFile(record, []byte(fmt.Sprintf("%x", sha256.Sum256(data))), 0600)
}

// WriteTrustedFile writes configuration produced by a host configuration command.
func WriteTrustedFile(path string, data []byte) error {
	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}
	if IsLocalPolicy(path) {
		return trustContents(path, data)
	}
	return nil
}

// IsLocalPolicy identifies project configuration and profile files.
func IsLocalPolicy(path string) bool {
	path = filepath.Clean(path)
	parent := filepath.Dir(path)
	name := filepath.Base(path)
	return name == ".bws.jsonc" ||
		(filepath.Base(parent) == ".bws" && (name == "config.jsonc" || name == "config.json")) ||
		(filepath.Base(parent) == "profiles" && filepath.Base(filepath.Dir(parent)) == ".bws" &&
			(strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".jsonc")))
}

// TrustWorkspace approves the local config and profiles after explicit user review.
func TrustWorkspace(dir string) error {
	root, path := FindWorkspaceRoot(dir)
	if _, err := os.Stat(path); err == nil {
		if _, err := LoadFile(path); err != nil {
			return err
		}
		if err := TrustFile(path); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	profiles := filepath.Join(root, ".bws", "profiles")
	entries, err := os.ReadDir(profiles)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() && (strings.HasSuffix(entry.Name(), ".json") || strings.HasSuffix(entry.Name(), ".jsonc")) {
			if err := TrustFile(filepath.Join(profiles, entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

// LoadLocalFile refuses local configuration that has not been reviewed on the host.
func LoadLocalFile(path string) (*Config, error) {
	data, err := ReadTrustedFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data, path)
}
