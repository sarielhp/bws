package sandbox

import (
	_ "embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"bws/internal/config"
	"bws/internal/util"
)

// DefaultTmuxConf is the built-in .tmux.conf of the sandbox home.
//
//go:embed embedded_skeleton/tmux.conf
var DefaultTmuxConf string

// DefaultBashrc is the built-in .bashrc of the sandbox home.
//
//go:embed embedded_skeleton/bashrc
var DefaultBashrc string

// DefaultProfile is the built-in .profile of the sandbox home.
//
//go:embed embedded_skeleton/profile
var DefaultProfile string

// SkeletonMapping maps an embedded asset file to its real dotfile target name in the sandbox home.
type SkeletonMapping struct {
	AssetName  string
	TargetName string
	Content    *string
}

// SkeletonFileMappings returns the table mapping embedded asset names to their staged dotfile names.
func SkeletonFileMappings() []SkeletonMapping {
	return []SkeletonMapping{
		{AssetName: "tmux.conf", TargetName: ".tmux.conf", Content: &DefaultTmuxConf},
		{AssetName: "bashrc", TargetName: ".bashrc", Content: &DefaultBashrc},
		{AssetName: "profile", TargetName: ".profile", Content: &DefaultProfile},
	}
}

// GlobalSkeletonDir returns ~/.config/bws/skeleton, the user's sandbox dotfiles.
func GlobalSkeletonDir() string {
	return filepath.Join(config.Dir(), "skeleton")
}

// LocalSkeletonDir returns the workspace skeleton directory under cwd.
func LocalSkeletonDir(cwd string) string {
	return filepath.Join(cwd, ".bw", "skeleton")
}

// EnsureGlobalSkeleton writes any missing built-in dotfiles into GlobalSkeletonDir.
func EnsureGlobalSkeleton() error {
	dir := GlobalSkeletonDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating skeleton directory: %w", err)
	}

	for _, f := range SkeletonFileMappings() {
		target := filepath.Join(dir, f.TargetName)
		if _, err := os.Stat(target); os.IsNotExist(err) {
			if err := os.WriteFile(target, []byte(*f.Content), 0644); err != nil {
				return fmt.Errorf("writing skeleton file %s: %w", target, err)
			}
		}
	}
	return nil
}

// StageHome creates a temporary sandbox home from the skeletons and returns it
// with a function that removes it.
func StageHome(cfg *config.Config, currentDir string) (string, func(), error) {
	if err := EnsureGlobalSkeleton(); err != nil {
		return "", nil, err
	}

	stageDir, err := util.UserTempDir("stage")
	if err != nil {
		return "", nil, fmt.Errorf("creating session stage directory: %w", err)
	}

	cleanup := func() {
		os.RemoveAll(stageDir)
	}

	// 1. Copy global skeleton
	globalSkel := GlobalSkeletonDir()
	if err := copyDirectoryContents(globalSkel, stageDir); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("copying global skeleton: %w", err)
	}

	for _, rcName := range []string{".bashrc", ".zshrc"} {
		stagedRC := filepath.Join(stageDir, rcName)
		localSkel := LocalSkeletonDir(currentDir)
		localRCExists := false
		if fi, err := os.Stat(filepath.Join(localSkel, rcName)); err == nil && !fi.IsDir() {
			localRCExists = true
		}

		if fi, err := os.Stat(stagedRC); err == nil && !fi.IsDir() && !localRCExists {
			data, err := os.ReadFile(stagedRC)
			if err == nil {
				comment := fmt.Sprintf("# --- Copied from global skeleton (%s) ---\n", filepath.Join(globalSkel, rcName))
				os.WriteFile(stagedRC, append([]byte(comment), data...), 0644)
			}
		}
	}

	// 2. Overlay project skeleton if present
	localSkel := LocalSkeletonDir(currentDir)
	if fi, err := os.Stat(localSkel); err == nil && fi.IsDir() {
		if err := copyLocalSkeleton(currentDir, stageDir); err != nil {
			cleanup()
			return "", nil, fmt.Errorf("copying local skeleton: %w", err)
		}
		for _, rcName := range []string{".bashrc", ".zshrc"} {
			stagedRC := filepath.Join(stageDir, rcName)
			if fi, err := os.Stat(filepath.Join(localSkel, rcName)); err == nil && !fi.IsDir() {
				data, err := os.ReadFile(stagedRC)
				if err == nil {
					comment := fmt.Sprintf("# --- Copied from local skeleton (%s) ---\n", filepath.Join(localSkel, rcName))
					os.WriteFile(stagedRC, append([]byte(comment), data...), 0644)
				}
			}
		}
	}

	// 3. Inject dynamic appendices into staged .bashrc
	stagedBashrc := filepath.Join(stageDir, ".bashrc")
	if err := appendDynamicConfig(cfg, stagedBashrc); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("appending dynamic shell config: %w", err)
	}

	// 4. Pre-create mountpoint paths inside staged home
	precreateMountpoints(cfg, stageDir, currentDir, util.HomeDir())

	// 5. Replicate host home convenience symlinks
	replicateHomeSymlinks(stageDir, util.HomeDir())

	return stageDir, cleanup, nil
}

func replicateHomeSymlinks(stageDir, home string) {
	entries, err := os.ReadDir(home)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.Type()&os.ModeSymlink == 0 {
			continue
		}
		name := e.Name()
		dest := filepath.Join(stageDir, name)
		if _, err := os.Lstat(dest); err == nil {
			continue
		}
		src := filepath.Join(home, name)
		target, err := os.Readlink(src)
		if err != nil {
			continue
		}
		_ = os.Symlink(target, dest)
	}
}

// precreateMountpoints creates, in the staged home, the directories and empty
// files that the workspace, binds and masks under home are mounted onto.
func precreateMountpoints(cfg *config.Config, stageDir, currentDir, home string) {
	wsRoot, _ := config.FindWorkspaceRoot(currentDir)
	for _, d := range []string{currentDir, wsRoot} {
		if rel, ok := strings.CutPrefix(d, home+"/"); ok {
			_ = os.MkdirAll(filepath.Join(stageDir, rel), 0755) // best effort: bwrap reports a missing mount point
		}
	}

	resolvePath := func(p string) string {
		p = strings.ReplaceAll(p, config.HomeToken, home)
		p = util.ExpandHome(p)
		if !filepath.IsAbs(p) && currentDir != "" {
			p = filepath.Join(currentDir, p)
		}
		return filepath.Clean(p)
	}

	for _, b := range slices.Concat(cfg.BindsRW, cfg.BindsRO) {
		dest := b.Sandbox
		if dest == "" {
			dest = b.Host
		}
		stageMountpoint(stageDir, home, resolvePath(dest), resolvePath(b.Host))
	}

	masks := slices.Clone(cfg.Mask)
	if config.HistoryMaskEnabled(cfg) {
		masks = append(masks, config.DefaultHistoryMasks...)
	}
	seen := make(map[string]bool)
	for _, m := range masks {
		expanded := resolvePath(m)
		if !seen[expanded] {
			seen[expanded] = true
			stageMountpoint(stageDir, home, expanded, expanded)
		}
	}
}

// stageMountpoint creates the mount point for dest inside the staged home when
// dest is under home: a directory or an empty file, matching the host path.
// Nothing is created when the host path does not exist. Errors are ignored:
// bwrap reports a mount point it cannot use.
func stageMountpoint(stageDir, home, dest, host string) {
	rel, ok := strings.CutPrefix(dest, home+"/")
	if !ok {
		return
	}
	fi, err := os.Stat(host)
	if err != nil {
		return
	}
	target := filepath.Join(stageDir, rel)
	if fi.IsDir() {
		_ = os.MkdirAll(target, 0755)
		return
	}
	_ = os.MkdirAll(filepath.Dir(target), 0755)
	if _, err := os.Stat(target); os.IsNotExist(err) {
		_ = os.WriteFile(target, []byte{}, 0644)
	}
}

func copyDirectoryContents(srcDir, destDir string) error {
	root, err := os.OpenRoot(srcDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer root.Close()
	return copyRootContents(root, ".", destDir)
}

func copyLocalSkeleton(projectDir, destDir string) error {
	project, err := os.OpenRoot(projectDir)
	if err != nil {
		return err
	}
	defer project.Close()
	root, err := project.OpenRoot(filepath.Join(".bw", "skeleton"))
	if err != nil {
		return err
	}
	defer root.Close()
	return copyRootContents(root, ".", destDir)
}

func copyRootContents(root *os.Root, dir, destDir string) error {
	entries, err := fs.ReadDir(root.FS(), dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(dir, entry.Name())
		destPath := filepath.Join(destDir, entry.Name())

		if entry.IsDir() {
			if err := os.MkdirAll(destPath, 0755); err != nil {
				return err
			}
			if err := copyRootContents(root, srcPath, destPath); err != nil {
				return err
			}
		} else {
			if err := copyRootFile(root, srcPath, destPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyRootFile(root *os.Root, src, dest string) error {
	info, err := root.Stat(src)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("skeleton entry %s must resolve to a regular file inside the skeleton directory", src)
	}
	in, err := root.OpenFile(src, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err = in.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("skeleton entry %s is not a regular file", src)
	}

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func appendDynamicConfig(cfg *config.Config, bashrcPath string) error {
	f, err := os.OpenFile(bashrcPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	var sb strings.Builder
	sb.WriteString("\n# --- Dynamic Sandbox Appendices (Generated by bws from configuration file) ---\n")

	if len(cfg.Path) > 0 {
		resolved := make([]string, 0, len(cfg.Path))
		for _, p := range cfg.Path {
			resolvedP := strings.ReplaceAll(p, config.HomeToken, util.HomeDir())
			resolvedP = util.ExpandHome(resolvedP)
			resolved = append(resolved, resolvedP)
		}
		sb.WriteString("\n# Add sandbox-specific binary paths (Generated by bws from configuration file 'path' entry)\n")
		sb.WriteString(fmt.Sprintf("export PATH=\"%s:$PATH\"\n", strings.Join(resolved, ":")))
	}

	if cfg.Env != nil {
		hasEnv := false
		var envSB strings.Builder
		if editor, ok := cfg.Env["EDITOR"]; ok && editor != "" {
			envSB.WriteString(fmt.Sprintf("export EDITOR=%q\n", editor))
			hasEnv = true
		}
		if visual, ok := cfg.Env["VISUAL"]; ok && visual != "" {
			envSB.WriteString(fmt.Sprintf("export VISUAL=%q\n", visual))
			hasEnv = true
		}
		if gopath, ok := cfg.Env["GOPATH"]; ok && gopath != "" {
			envSB.WriteString(fmt.Sprintf("export GOPATH=%q\n", gopath))
			hasEnv = true
		}
		if hasEnv {
			sb.WriteString("\n# sandbox default environment variables (Generated by bws from configuration file 'env' entry)\n")
			sb.WriteString(envSB.String())
		}
	}

	_, err = f.WriteString(sb.String())
	return err
}
