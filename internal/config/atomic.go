package config

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"

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
	if err := backupConfig(path); err != nil {
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
	fi, err := os.Lstat(abs)
	if err == nil {
		if fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir() {
			return nil, fmt.Errorf("refusing symlink or non-directory policy parent %s", filepath.Base(abs))
		}
		return os.OpenRoot(abs)
	}
	if !os.IsNotExist(err) {
		return nil, err
	}

	ancestor, parts, err := findExistingAncestor(abs)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(ancestor)
	if err != nil {
		return nil, err
	}
	return descendPolicyParts(root, parts)
}

func findExistingAncestor(abs string) (string, []string, error) {
	curr := abs
	var parts []string
	for {
		parent := filepath.Dir(curr)
		if parent == curr {
			return "", nil, fmt.Errorf("no existing ancestor for %s", abs)
		}
		parts = append([]string{filepath.Base(curr)}, parts...)
		curr = parent

		fi, err := os.Stat(curr)
		if err == nil {
			if !fi.IsDir() {
				return "", nil, fmt.Errorf("refusing non-directory policy ancestor %s", filepath.Base(curr))
			}
			return curr, parts, nil
		}
		if !os.IsNotExist(err) {
			return "", nil, err
		}
	}
}

func descendPolicyParts(root *os.Root, parts []string) (*os.Root, error) {
	for _, part := range parts {
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

// atomicWriteFile replaces path with data via a synced temp file and rename,
// so a crash leaves either the old or the new contents. A symlinked path is
// resolved first so the link itself is preserved, and an existing file keeps
// its permissions.
func atomicWriteFile(path string, data []byte) error {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	perm := os.FileMode(0644)
	if fi, err := os.Stat(path); err == nil {
		perm = fi.Mode().Perm()
	}
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(perm); err != nil {
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
	return os.Rename(tmp, path)
}
