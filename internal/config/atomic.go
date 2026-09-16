package config

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// PolicyBytes reads regular policy files without following a final symlink.
func PolicyBytes(path string) ([]byte, error) {
	root, err := os.OpenRoot(filepath.Dir(path))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer root.Close()
	f, err := root.OpenFile(filepath.Base(path), os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("policy destination is not a regular file: %s", path)
	}
	return io.ReadAll(f)
}

// AtomicPolicyWrite writes reviewed bytes, refusing stale previews and symlinks.
// A nil expected value means create-only. Cooperating writers share a lock.
func AtomicPolicyWrite(path string, data, expected []byte) error {
	root, err := policyDirectory(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer root.Close()
	lock, err := root.OpenFile(".write.lock", os.O_CREATE|os.O_RDWR|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return fmt.Errorf("another policy write is in progress: %w", err)
	}
	defer unix.Flock(int(lock.Fd()), unix.LOCK_UN)
	name := filepath.Base(path)
	if err := checkExpected(root, name, expected); err != nil {
		return err
	}
	temp := fmt.Sprintf(".write-%x", rand.Text())
	f, err := root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer root.Remove(temp)
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if expected == nil {
		err = root.Link(temp, name)
	} else {
		err = root.Rename(temp, name)
	}
	if err != nil {
		return err
	}
	if IsLocalPolicy(path) {
		return trustContents(path, data)
	}
	return nil
}

func checkExpected(root *os.Root, name string, expected []byte) error {
	fi, err := root.Lstat(name)
	if os.IsNotExist(err) && expected == nil {
		return nil
	}
	if err != nil {
		return fmt.Errorf("policy changed since preview: %w", err)
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("refusing non-regular policy destination %s", name)
	}
	if expected == nil {
		return fmt.Errorf("%s already exists; use --force to replace", name)
	}
	data, err := root.ReadFile(name)
	if err != nil {
		return err
	}
	if !bytes.Equal(data, expected) {
		return fmt.Errorf("%s changed since preview; retry after reviewing", name)
	}
	return nil
}

func policyDirectory(dir string) (*os.Root, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot("/")
	if err != nil {
		return nil, err
	}
	for _, part := range strings.Split(strings.TrimPrefix(abs, "/"), "/") {
		if part == "" {
			continue
		}
		fi, err := root.Lstat(part)
		if os.IsNotExist(err) {
			err = root.Mkdir(part, 0755)
		} else if err == nil && !fi.IsDir() {
			err = fmt.Errorf("refusing symlink or non-directory policy parent %s", part)
		}
		if err != nil {
			root.Close()
			return nil, err
		}
		next, err := root.OpenRoot(part)
		root.Close()
		if err != nil {
			return nil, err
		}
		root = next
	}
	return root, nil
}
