package main

import (
	"os"
	"path"
	"path/filepath"
	"strings"
)

// gitignorePattern is one parsed line from a .gitignore (or
// .git/info/exclude) file.
//
// Supported syntax (documented precisely in DETAILS.md):
//   - blank lines and lines starting with '#' are ignored
//   - a leading '!' negates the pattern (re-includes a previously excluded
//     path)
//   - a leading '/' anchors the pattern to the directory containing the
//     gitignore file (rather than matching at any depth below it)
//   - a trailing '/' restricts the pattern to directories only — since
//     dotenvleak only ever matches plain files, such patterns are still
//     evaluated as a plain path-segment match; the directory-only
//     restriction only means dotenvleak will not treat it as matching a
//     file whose own name equals the pattern, which is already default
//     glob behaviour here
//   - '*' matches any run of characters except '/' within one path segment
//   - '?' matches any single character except '/'
//   - '**' as a whole path segment matches zero or more path segments
//     (leading "**/x", trailing "x/**", and interior "a/**/b" are all
//     supported)
//   - a pattern containing a '/' anywhere except a single trailing slash is
//     anchored to the gitignore's directory, same as real git; a pattern
//     with no '/' at all matches the basename at any depth below that
//     directory
//
// NOT supported (documented, deliberate scope limits — see DETAILS.md):
//   - character classes like "[aA]bc" or "[!ab]" are treated as literal
//     characters rather than expanded
//   - escaping a leading '!' or '#' with a backslash is not implemented
//   - per-line trailing-whitespace / backslash-escaping edge cases beyond a
//     simple TrimRight of unescaped spaces
type gitignorePattern struct {
	raw       string
	negate    bool
	anchored  bool // pattern is relative to sourceDir, not matchable at any depth
	dirOnly   bool
	segments  []string // pattern split on '/', with anchoring slashes stripped
	sourceDir string   // absolute dir of the .gitignore file that defined this pattern (forward-slash form)
}

// parseGitignore parses the contents of one gitignore-style file. sourceDir
// is the absolute directory (forward-slash form) that the file lives in,
// used to anchor '/'-containing patterns.
func parseGitignore(content, sourceDir string) []gitignorePattern {
	var out []gitignorePattern
	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimRight(rawLine, " \t\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		p := gitignorePattern{raw: line, sourceDir: sourceDir}
		if strings.HasPrefix(line, "!") {
			p.negate = true
			line = line[1:]
		}
		if strings.HasSuffix(line, "/") {
			p.dirOnly = true
			line = strings.TrimSuffix(line, "/")
		}
		if line == "" {
			continue
		}
		if strings.Contains(line, "/") {
			p.anchored = true
			line = strings.TrimPrefix(line, "/")
		}
		p.segments = strings.Split(line, "/")
		out = append(out, p)
	}
	return out
}

// matches reports whether this pattern matches relFromSource, the path of a
// candidate file relative to the pattern's sourceDir (forward-slash form,
// no leading slash).
func (p gitignorePattern) matches(relFromSource string) bool {
	pathSegs := strings.Split(relFromSource, "/")

	if !p.anchored {
		// Unanchored, single-segment pattern: matches the basename at any
		// depth. Match it against the final path segment only.
		return globSegment(p.segments[len(p.segments)-1], pathSegs[len(pathSegs)-1])
	}
	return matchSegments(p.segments, pathSegs)
}

// matchSegments matches a '/'-anchored pattern's segments against a path's
// segments, supporting "**" as a segment that consumes zero or more path
// segments.
func matchSegments(pattern, pathSegs []string) bool {
	if len(pattern) == 0 {
		return len(pathSegs) == 0
	}
	if pattern[0] == "**" {
		if len(pattern) == 1 {
			return true // trailing "**" matches everything below
		}
		for i := 0; i <= len(pathSegs); i++ {
			if matchSegments(pattern[1:], pathSegs[i:]) {
				return true
			}
		}
		return false
	}
	if len(pathSegs) == 0 {
		return false
	}
	if !globSegment(pattern[0], pathSegs[0]) {
		return false
	}
	return matchSegments(pattern[1:], pathSegs[1:])
}

// globSegment matches a single path segment against a single pattern
// segment using '*' and '?' glob semantics (no '/' involved at this level).
func globSegment(pattern, segment string) bool {
	ok, err := path.Match(pattern, segment)
	if err != nil {
		return pattern == segment
	}
	return ok
}

// gitignoreChain collects the ordered list of gitignore files that apply to
// a file at fileDir (absolute, forward-slash), walking from repoRoot down
// to fileDir, plus repoRoot's .git/info/exclude. Order is root-to-leaf so
// that, matching real git precedence, patterns from a .gitignore closer to
// the file are evaluated after (and so can override) patterns from a
// .gitignore higher up the tree.
func gitignoreChain(repoRoot, fileDir string) []gitignorePattern {
	repoRoot = filepath.ToSlash(repoRoot)
	fileDir = filepath.ToSlash(fileDir)

	var dirs []string
	dirs = append(dirs, repoRoot)
	if fileDir != repoRoot {
		rel := strings.TrimPrefix(fileDir, repoRoot+"/")
		acc := repoRoot
		for _, seg := range strings.Split(rel, "/") {
			if seg == "" {
				continue
			}
			acc = acc + "/" + seg
			dirs = append(dirs, acc)
		}
	}

	var patterns []gitignorePattern

	if data, err := os.ReadFile(filepath.Join(filepath.FromSlash(repoRoot), ".git", "info", "exclude")); err == nil {
		patterns = append(patterns, parseGitignore(string(data), repoRoot)...)
	}

	for _, dir := range dirs {
		giPath := filepath.Join(filepath.FromSlash(dir), ".gitignore")
		data, err := os.ReadFile(giPath)
		if err != nil {
			continue
		}
		patterns = append(patterns, parseGitignore(string(data), dir)...)
	}

	return patterns
}

// isIgnoredByChain reports whether absFilePath is excluded by the given
// ordered pattern chain, applying git's "last matching pattern wins"
// semantics across the whole chain (root-to-leaf, in-file order).
func isIgnoredByChain(patterns []gitignorePattern, absFilePath string) bool {
	absFilePath = filepath.ToSlash(absFilePath)
	ignored := false
	for _, p := range patterns {
		relFromSource := strings.TrimPrefix(absFilePath, p.sourceDir+"/")
		if relFromSource == absFilePath && p.sourceDir != "" {
			// The file is not below this pattern's source directory at
			// all (shouldn't normally happen given how the chain is
			// built, but guard against it defensively).
			continue
		}
		if p.matches(relFromSource) {
			ignored = !p.negate
		}
	}
	return ignored
}

// isEffectivelyGitignored is the top-level entry point: does any
// .gitignore in the applicable chain for absFilePath (inside repoRoot)
// actually exclude it?
func isEffectivelyGitignored(repoRoot, absFilePath string) bool {
	fileDir := filepath.ToSlash(filepath.Dir(absFilePath))
	patterns := gitignoreChain(repoRoot, fileDir)
	return isIgnoredByChain(patterns, absFilePath)
}
