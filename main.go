package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

const usage = `usage: dotenvleak scan <repo-dir> [--json] [--strict] [--template-suffix=.ext] [--exclude-dir=name]

  scan <repo-dir>          scan the given git repo for leak-prone .env* files
  --json                   emit machine-readable JSON instead of text
  --strict                 also fail (exit 1) on DL003 (low-confidence) findings
  --template-suffix=.ext   add a filename suffix treated as a safe template file
                           (repeatable; replaces the built-in list on first use)
  --exclude-dir=name       add a directory name to skip while walking the repo
                           (repeatable; replaces the built-in list on first use)

Exit codes:
  0  no blocker findings (DL001/DL002), and no DL003 findings if --strict
  1  at least one blocker finding, or a DL003 finding under --strict
  2  usage error (not a git repository, directory not found, bad flags)
`

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 || args[0] != "scan" {
		fmt.Fprint(stderr, usage)
		return 2
	}

	repoDirArg := args[1]
	opts := DefaultOptions()
	jsonOut := false

	customTemplateSuffixes := false
	customExcludeDirs := false

	for _, a := range args[2:] {
		switch {
		case a == "--json":
			jsonOut = true
		case a == "--strict":
			opts.Strict = true
		case strings.HasPrefix(a, "--template-suffix="):
			v := strings.TrimPrefix(a, "--template-suffix=")
			if !customTemplateSuffixes {
				opts.TemplateSuffixes = nil
				customTemplateSuffixes = true
			}
			opts.TemplateSuffixes = append(opts.TemplateSuffixes, v)
		case strings.HasPrefix(a, "--exclude-dir="):
			v := strings.TrimPrefix(a, "--exclude-dir=")
			if !customExcludeDirs {
				opts.ExcludeDirs = nil
				customExcludeDirs = true
			}
			opts.ExcludeDirs = append(opts.ExcludeDirs, v)
		default:
			fmt.Fprintf(stderr, "unknown flag: %s\n\n%s", a, usage)
			return 2
		}
	}

	info, err := os.Stat(repoDirArg)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if !info.IsDir() {
		fmt.Fprintf(stderr, "%s is not a directory\n", repoDirArg)
		return 2
	}

	if !isGitRepo(repoDirArg) {
		fmt.Fprintf(stderr, "%s is not a git repository\n", repoDirArg)
		return 2
	}

	repoRoot, err := gitRepoRoot(repoDirArg)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	findings, err := Check(repoRoot, opts)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	if jsonOut {
		out, err := RenderJSON(findings)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		fmt.Fprintln(stdout, out)
	} else {
		fmt.Fprintln(stdout, RenderText(findings))
	}

	fail := false
	for _, f := range findings {
		if f.Severity == Blocker {
			fail = true
		}
		if opts.Strict && f.Rule == DL003 {
			fail = true
		}
	}
	if fail {
		return 1
	}
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
