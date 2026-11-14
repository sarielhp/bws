package bwrap

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"bws/internal/config"
	"bws/internal/util"
)

// maxModuleDepth bounds the recursion into nested submodule Git directories.
const maxModuleDepth = 8

// gitGuard collects the mounts that keep a workspace's executable Git state
// read-only. Host git runs hooks and honours core.fsmonitor, core.pager,
// aliases and similar settings from these files, so a sandbox able to write
// them could run code on the host the next time the user runs git there.
//
// A read-only mount alone is not enough: a directory that merely contains a
// mount point can still be renamed (mv .git .git2) and replaced by a writable
// copy. Mount points themselves cannot be renamed or removed, so every
// directory between the workspace mount and a protected path is pinned by
// binding it onto itself.
//
// Mounts are collected first and emitted parents-first, because bwrap applies
// them in order and a later bind of a directory hides earlier mounts below it.
type gitGuard struct {
	roots    []string
	pins     map[string]bool   // directories bound onto themselves
	readOnly map[string]string // destination -> source
	why      map[string]string
	dryRun   bool
}

type guardMount struct{ flag, src, dst, why string }

func addGitProtectArgs(args *[]string, cfg *config.Config, currentDir string, dryRun, verbose bool) {
	if !config.ProtectGit(cfg) || currentDir == "" {
		return
	}
	g := &gitGuard{
		pins: map[string]bool{}, readOnly: map[string]string{}, why: map[string]string{},
		dryRun: dryRun,
	}
	g.roots = []string{filepath.Clean(currentDir)}
	if wsRoot, _ := config.FindWorkspaceRoot(currentDir); wsRoot != "" && wsRoot != "/" && filepath.Clean(wsRoot) != g.roots[0] {
		g.roots = append(g.roots, filepath.Clean(wsRoot))
	}
	// Longest root first, so containingRoot picks the innermost mount.
	sort.Slice(g.roots, func(i, j int) bool { return len(g.roots[i]) > len(g.roots[j]) })
	for _, root := range g.roots {
		g.protectWorktree(root)
	}
	for _, m := range g.mounts() {
		*args = append(*args, m.flag, m.src, m.dst)
		if verbose {
			fmt.Fprintf(os.Stderr, "[verbose]   %s %s %s (%s)\n", m.flag, m.src, m.dst, m.why)
		}
	}
}

// protectWorktree handles root/.git as either a Git directory or a gitdir
// file (linked worktree or submodule checkout).
func (g *gitGuard) protectWorktree(root string) {
	dotGit := filepath.Join(root, ".git")
	fi, err := os.Lstat(dotGit)
	if err != nil {
		return
	}
	if fi.IsDir() {
		g.protectGitDir(dotGit, root, 0)
		return
	}
	if !fi.Mode().IsRegular() {
		return
	}
	gitDir := readGitDirFile(dotGit)
	if gitDir == "" {
		return
	}
	g.protectGitDir(gitDir, root, 0)
	if common := readCommonDir(gitDir); common != "" {
		g.protectGitDir(common, root, 0)
	}
}

// protectGitDir pins gitDir and makes its config files and hooks read-only.
// Git directories outside the workspace mounts are left alone: binding them
// would expose host state the sandbox cannot otherwise see.
func (g *gitGuard) protectGitDir(gitDir, worktree string, depth int) {
	gitDir = filepath.Clean(gitDir)
	if depth > maxModuleDepth || g.containingRoot(gitDir) == "" || !dirExists(gitDir) {
		return
	}
	g.pin(gitDir)
	for _, name := range []string{"config", "config.worktree"} {
		cfgFile := filepath.Join(gitDir, name)
		if !fileExists(cfgFile) {
			continue
		}
		g.addReadOnly(cfgFile, cfgFile, "git config")
		g.protectConfigRefs(cfgFile, worktree)
	}
	g.protectDir(filepath.Join(gitDir, "hooks"), "git hooks")
	g.protectModules(filepath.Join(gitDir, "modules"), depth)
}

// protectModules finds submodule Git directories under modules/. Submodule
// names may contain slashes, so any directory holding a HEAD file counts.
func (g *gitGuard) protectModules(modulesDir string, depth int) {
	entries, err := os.ReadDir(modulesDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sub := filepath.Join(modulesDir, e.Name())
		if fileExists(filepath.Join(sub, "HEAD")) {
			g.protectGitDir(sub, "", depth+1)
		} else {
			g.protectModules(sub, depth+1)
		}
	}
}

// protectConfigRefs protects files that cfgFile points to inside the
// workspace: a core.hooksPath directory and include/includeIf targets.
func (g *gitGuard) protectConfigRefs(cfgFile, worktree string) {
	for _, kv := range readGitConfigPaths(cfgFile) {
		isHooks := kv[0] == "core.hookspath"
		p := expandGitPath(kv[1], filepath.Dir(cfgFile))
		if isHooks {
			p = expandGitPath(kv[1], worktree)
		}
		if p == "" || g.containingRoot(p) == "" {
			continue
		}
		if isHooks {
			g.protectDir(p, "git core.hooksPath")
		} else {
			g.protectFile(p, "git include")
		}
	}
}

// protectDir mounts dir read-only, creating it on the host first so the
// sandbox cannot create it later.
func (g *gitGuard) protectDir(dir, why string) {
	if !dirExists(dir) && !g.dryRun {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "[bws] Warning: cannot create %s to protect it: %v\n", dir, err)
			return
		}
	}
	g.pinAncestors(dir)
	g.addReadOnly(dir, dir, why)
}

// protectFile mounts file read-only, or an empty read-only file when it does
// not exist yet.
func (g *gitGuard) protectFile(file, why string) {
	src := file
	if !fileExists(file) {
		if _, err := os.Lstat(file); err == nil {
			return // a directory or special file; git cannot include it
		}
		src = "/dev/null"
	}
	g.pinAncestors(file)
	g.addReadOnly(src, file, why)
}

func (g *gitGuard) addReadOnly(src, dst, why string) {
	g.readOnly[dst] = src
	g.why[dst] = why
}

// pin binds dir onto itself, along with its ancestors.
func (g *gitGuard) pin(dir string) {
	g.pinAncestors(dir)
	g.pins[dir] = true
}

// pinAncestors pins every directory strictly between the containing
// workspace root and p.
func (g *gitGuard) pinAncestors(p string) {
	root := g.containingRoot(p)
	if root == "" {
		return
	}
	for d := filepath.Dir(p); d != root && strings.HasPrefix(d, root+"/"); d = filepath.Dir(d) {
		g.pins[d] = true
	}
}

// mounts returns the collected mounts, parents before children. A pin is
// dropped when it is, or lies inside, a read-only target: that path is
// already a mount point or read-only, and a writable bind would undo it.
// A read-only target inside another one is dropped as already covered;
// bwrap could not create its mount point there anyway.
func (g *gitGuard) mounts() []guardMount {
	var out []guardMount
	for dst, src := range g.readOnly {
		if !g.underReadOnly(filepath.Dir(dst)) {
			out = append(out, guardMount{"--ro-bind", src, dst, g.why[dst]})
		}
	}
	for dir := range g.pins {
		if !g.underReadOnly(dir) {
			out = append(out, guardMount{"--bind", dir, dir, "pinned against rename"})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		di, dj := strings.Count(out[i].dst, "/"), strings.Count(out[j].dst, "/")
		if di != dj {
			return di < dj
		}
		return out[i].dst < out[j].dst
	})
	return out
}

func (g *gitGuard) underReadOnly(p string) bool {
	for ro := range g.readOnly {
		if p == ro || strings.HasPrefix(p, ro+"/") {
			return true
		}
	}
	return false
}

// containingRoot returns the innermost workspace mount strictly containing p.
func (g *gitGuard) containingRoot(p string) string {
	p = filepath.Clean(p)
	for _, r := range g.roots {
		if strings.HasPrefix(p, r+"/") {
			return r
		}
	}
	return ""
}

// readGitDirFile parses a ".git" file of the form "gitdir: <path>".
func readGitDirFile(dotGit string) string {
	data, err := os.ReadFile(dotGit)
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(strings.SplitN(string(data), "\n", 2)[0])
	rest, ok := strings.CutPrefix(line, "gitdir:")
	if !ok {
		return ""
	}
	return expandGitPath(strings.TrimSpace(rest), filepath.Dir(dotGit))
}

// readCommonDir resolves the commondir file of a linked worktree's Git dir.
func readCommonDir(gitDir string) string {
	data, err := os.ReadFile(filepath.Join(gitDir, "commondir"))
	if err != nil {
		return ""
	}
	return expandGitPath(strings.TrimSpace(string(data)), gitDir)
}

// readGitConfigPaths returns [key, value] pairs for core.hooksPath and
// include paths in a single config file. Includes are not followed and git
// config --file runs no hooks, so reading an untrusted file is safe.
func readGitConfigPaths(cfgFile string) [][2]string {
	if !util.CommandExists("git") {
		return nil
	}
	out, err := exec.Command("git", "config", "--file", cfgFile, "--null", "--get-regexp",
		`^(core\.hookspath|include\.path|includeif\..*\.path)$`).Output()
	if err != nil {
		return nil
	}
	var pairs [][2]string
	for _, entry := range strings.Split(string(out), "\x00") {
		key, val, ok := strings.Cut(entry, "\n")
		if ok && val != "" {
			pairs = append(pairs, [2]string{strings.ToLower(key), val})
		}
	}
	return pairs
}

// expandGitPath resolves ~ and relative paths the way git does for path
// values; relative paths are taken against base.
func expandGitPath(p, base string) string {
	if p == "" {
		return ""
	}
	p = util.ExpandHome(p)
	if !filepath.IsAbs(p) {
		if base == "" {
			return ""
		}
		p = filepath.Join(base, p)
	}
	return filepath.Clean(p)
}
