package main

import (
	"os"
	"path/filepath"
	"strings"
)

// defaultExcludeDirs are directory names that are skipped entirely while
// walking the repo looking for .env* files. These are the usual large,
// vendored, or VCS-internal trees where a matching filename would never be
// a real application secrets file worth flagging.
var defaultExcludeDirs = []string{
	"node_modules",
	"vendor",
	".git",
}

// defaultTemplateSuffixes are filename suffixes that mark a .env* file as a
// checked-in TEMPLATE rather than a real secrets file. Template files such
// as ".env.example" are meant to be committed: they document which keys an
// application expects, populated with placeholder values, precisely so
// developers can copy them to a real (gitignored) ".env" locally. Flagging
// them as leaks would be a false positive on an intentional, safe pattern.
var defaultTemplateSuffixes = []string{
	".example",
	".sample",
	".template",
	".dist",
}

// candidateFile is a discovered .env*-named file, prior to any tracked or
// content analysis.
type candidateFile struct {
	// AbsPath is the absolute path to the file on disk.
	AbsPath string
	// RelPath is the path relative to the scanned repo root, using forward
	// slashes, matching the form `git ls-files` reports paths in.
	RelPath string
}

// isEnvFilename reports whether a bare filename looks like a dotenv file,
// i.e. it is ".env" itself or starts with ".env" followed by a separator
// (".env.local", ".env.production") — not just any file with ".env"
// somewhere in the middle of its name.
func isEnvFilename(name string) bool {
	if name == ".env" {
		return true
	}
	return strings.HasPrefix(name, ".env.") || strings.HasPrefix(name, ".env-") || strings.HasPrefix(name, ".env_")
}

// isTemplateFile reports whether name ends in one of the configured
// template suffixes (case-insensitive), marking it as an intentionally
// committed, placeholder-only file that should be excluded from scanning.
func isTemplateFile(name string, templateSuffixes []string) bool {
	lower := strings.ToLower(name)
	for _, suf := range templateSuffixes {
		if suf == "" {
			continue
		}
		if strings.HasSuffix(lower, strings.ToLower(suf)) {
			return true
		}
	}
	return false
}

// FindEnvFiles walks repoRoot looking for files whose name matches the
// dotenv pattern, skipping excludeDirs entirely (by directory base name,
// anywhere in the tree) and skipping any file whose name ends in one of
// templateSuffixes.
func FindEnvFiles(repoRoot string, excludeDirs, templateSuffixes []string) ([]candidateFile, error) {
	excluded := make(map[string]bool, len(excludeDirs))
	for _, d := range excludeDirs {
		excluded[d] = true
	}

	var out []candidateFile
	err := filepath.WalkDir(repoRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// A file/dir might disappear mid-walk (e.g. a broken symlink);
			// skip it rather than aborting the whole scan.
			return nil
		}
		if d.IsDir() {
			if path != repoRoot && excluded[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !isEnvFilename(name) {
			return nil
		}
		if isTemplateFile(name, templateSuffixes) {
			return nil
		}
		rel, relErr := filepath.Rel(repoRoot, path)
		if relErr != nil {
			return nil
		}
		out = append(out, candidateFile{
			AbsPath: path,
			RelPath: filepath.ToSlash(rel),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
