package util

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// project names this program's tree of temporary files.
const project = "bws"

// TempPath returns <tmpdir>/<user>/bws/<purpose>, where bws keeps the
// temporary files made for purpose. It creates nothing; see TempPurposeDir.
func TempPath(purpose string) string {
	return filepath.Join(os.TempDir(), userName(), project, purpose) // tmp-exempt: the one helper that names the system temporary directory
}

// TempPurposeDir creates and returns TempPath(purpose). The user directory
// must belong to the user and be closed to writes by others; the project and
// purpose directories are private (0700). A directory planted by someone else
// is refused rather than used.
func TempPurposeDir(purpose string) (string, error) {
	dir := TempPath(purpose)
	project := filepath.Dir(dir)
	if err := ensureOwnedDir(filepath.Dir(project), false); err != nil {
		return "", err
	}
	if err := ensureOwnedDir(project, true); err != nil {
		return "", err
	}
	if err := ensureOwnedDir(dir, true); err != nil {
		return "", err
	}
	return dir, nil
}

// UserTempDir creates a new private directory for one run under
// TempPurposeDir(purpose), named <purpose>_<random>.
func UserTempDir(purpose string) (string, error) {
	base, err := TempPurposeDir(purpose)
	if err != nil {
		return "", err
	}
	return os.MkdirTemp(base, purpose+"_")
}

// UserTempFile creates a new temporary file under TempPurposeDir(purpose);
// pattern follows os.CreateTemp.
func UserTempFile(purpose, pattern string) (*os.File, error) {
	base, err := TempPurposeDir(purpose)
	if err != nil {
		return nil, err
	}
	return os.CreateTemp(base, pattern)
}

// ensureOwnedDir creates dir (0700) when missing, then checks that it is a
// real directory owned by the current user and not writable by others. A
// private directory must also be closed to the group and others; one of ours
// with looser permissions is tightened.
func ensureOwnedDir(dir string, private bool) error {
	if err := os.Mkdir(dir, 0o700); err != nil && !os.IsExist(err) {
		return fmt.Errorf("creating temporary directory %s: %w", dir, err)
	}
	fi, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("checking temporary directory %s: %w", dir, err)
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !fi.IsDir() || !ok || int(st.Uid) != os.Getuid() {
		return fmt.Errorf("refusing temporary directory %s: not a directory owned by the current user", dir)
	}
	if fi.Mode().Perm()&0o022 != 0 && !private {
		return fmt.Errorf("refusing temporary directory %s: writable by others", dir)
	}
	if private && fi.Mode().Perm()&0o077 != 0 {
		if err := os.Chmod(dir, 0o700); err != nil {
			return fmt.Errorf("making temporary directory %s private: %w", dir, err)
		}
	}
	return nil
}

// userName is the current user's login name, or the numeric uid when it
// cannot be looked up; it never contains a path separator.
func userName() string {
	name := ""
	if u, err := user.Current(); err == nil {
		name = u.Username
	}
	if name == "" {
		name = os.Getenv("USER")
	}
	if name == "" || strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
		name = strconv.Itoa(os.Getuid())
	}
	return name
}
