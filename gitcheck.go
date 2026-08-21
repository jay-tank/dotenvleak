package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// This file is the one deliberate exception, in this workspace's package
// lineup, to "purely static parsing, no shelling out to git". Whether a
// file is CURRENTLY tracked, and how gitignore rules apply, are git-defined
// concepts (git's own ignore/exclude precedence, .git/info/exclude, global
// excludesfile, sparse-checkout, etc. all factor in) that only `git` itself
// answers authoritatively. Re-implementing git's own tracking/ignore engine
// from scratch would be both a large undertaking and a worse, more
// bug-prone answer than just asking git. See DETAILS.md for the full
// rationale and for the exact scope of the pragmatic .gitignore pattern
// matcher dotenvleak implements itself for the "would this pattern match"
// question asked ahead of `git add` (i.e. for files not yet tracked).
type gitRunner struct {
	repoRoot string
}

func newGitRunner(repoRoot string) *gitRunner {
	return &gitRunner{repoRoot: repoRoot}
}

// isGitRepo reports whether dir is inside a git working tree.
func isGitRepo(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}

// gitRepoRoot returns the top-level directory of the git working tree that
// dir belongs to.
func gitRepoRoot(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository (or any parent up to the mount point): %s", dir)
	}
	return strings.TrimSpace(string(out)), nil
}

// isTracked reports whether relPath (relative to repoRoot, forward slashes)
// is currently tracked by git, using `git ls-files --error-unmatch`'s exit
// code as the source of truth.
func (g *gitRunner) isTracked(relPath string) (bool, error) {
	cmd := exec.Command("git", "ls-files", "--error-unmatch", "--", relPath)
	cmd.Dir = g.repoRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		// git ls-files --error-unmatch exits 1 when the path is not
		// tracked; that is an expected, non-error outcome for us.
		if exitErr.ExitCode() == 1 {
			return false, nil
		}
	}
	return false, fmt.Errorf("git ls-files failed for %s: %w (%s)", relPath, err, stderr.String())
}
