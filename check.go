package main

import "os"

// Options controls a Check run.
type Options struct {
	ExcludeDirs      []string
	TemplateSuffixes []string
	Strict           bool // also surface DL003 as part of the returned findings
}

// DefaultOptions returns the built-in default configuration.
func DefaultOptions() Options {
	return Options{
		ExcludeDirs:      append([]string(nil), defaultExcludeDirs...),
		TemplateSuffixes: append([]string(nil), defaultTemplateSuffixes...),
		Strict:           false,
	}
}

// Check scans repoRoot (which must be a git repository) for leak-prone
// .env* files and returns the findings.
func Check(repoRoot string, opts Options) ([]Finding, error) {
	candidates, err := FindEnvFiles(repoRoot, opts.ExcludeDirs, opts.TemplateSuffixes)
	if err != nil {
		return nil, err
	}

	git := newGitRunner(repoRoot)
	var findings []Finding

	for _, c := range candidates {
		tracked, err := git.isTracked(c.RelPath)
		if err != nil {
			return nil, err
		}

		content, readErr := os.ReadFile(c.AbsPath)
		if readErr != nil {
			// Unreadable file (permissions, race with deletion, etc.) —
			// skip rather than fail the whole scan.
			continue
		}

		assignments := parseEnvAssignments(string(content))
		secretAssignments := findSecretLikeAssignments(assignments)
		keys := keysOf(secretAssignments)

		switch {
		case len(secretAssignments) > 0 && tracked:
			findings = append(findings, newDL001(c.RelPath, keys))
		case len(secretAssignments) > 0 && !tracked:
			if !isEffectivelyGitignored(repoRoot, c.AbsPath) {
				findings = append(findings, newDL002(c.RelPath, keys))
			}
			// If it IS effectively gitignored and not tracked, there is
			// nothing to report: the protection is working as intended.
		default:
			// No clearly secret-looking content detected.
			if opts.Strict {
				findings = append(findings, newDL003(c.RelPath))
			}
		}
	}

	return findings, nil
}

func keysOf(assignments []envAssignment) []string {
	keys := make([]string, 0, len(assignments))
	seen := make(map[string]bool, len(assignments))
	for _, a := range assignments {
		if seen[a.Key] {
			continue
		}
		seen[a.Key] = true
		keys = append(keys, a.Key)
	}
	return keys
}
