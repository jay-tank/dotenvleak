# Usage

```
dotenvleak scan <repo-dir> [--json] [--strict] [--template-suffix=.ext] [--exclude-dir=name]
```

`<repo-dir>` must be a directory inside (or the root of) a git working tree.
`dotenvleak` resolves it to the repository's top-level directory via
`git rev-parse --show-toplevel` before scanning, so you can point it at any
subdirectory of a repo.

## Flags

- `--json` — emit findings as a JSON array instead of human-readable text.
  Empty result is `[]`.
- `--strict` — also treat `DL003` (low-confidence: a `.env*` file with no
  clearly secret-looking content) as a failing condition (exit 1). Off by
  default.
- `--template-suffix=.ext` — add a filename suffix that marks a `.env*`
  file as an intentionally-committed template, excluded from scanning.
  Repeatable; the **first** occurrence replaces the built-in default list
  (`.example`, `.sample`, `.template`, `.dist`), so pass all suffixes you
  want if you use this flag:
  ```sh
  dotenvleak scan . --template-suffix=.example --template-suffix=.default
  ```
- `--exclude-dir=name` — add a directory name to skip while walking the
  repo tree. Repeatable; the first occurrence replaces the built-in default
  list (`node_modules`, `vendor`, `.git`).

## Exit codes

| Code | Meaning |
|---|---|
| `0` | No blocker findings (and no `DL003` findings if `--strict` was passed) |
| `1` | At least one blocker finding (`DL001`/`DL002`), or a `DL003` finding under `--strict` |
| `2` | Usage error: not a git repository, directory not found, or an unrecognized flag |

## Findings

| Rule | Severity | Meaning |
|---|---|---|
| `DL001` | blocker | A `.env*` file with secret-looking content is currently tracked by git — already committed. |
| `DL002` | blocker | A `.env*` file with secret-looking content is untracked, and no applicable `.gitignore` pattern would exclude it — one `git add` away from `DL001`. |
| `DL003` | warning (opt-in) | A `.env*` file exists with no clearly secret-looking content detected. Only reported with `--strict`. |

Full rationale for the heuristics and the `.gitignore` matcher's exact
scope is in [../DETAILS.md](../DETAILS.md).

## Example output

Text (default):

```
$ dotenvleak scan .
[DL001] BLOCKER: .env is tracked by git and contains secret-looking value(s) for: API_KEY
[DL002] BLOCKER: services/worker/.env.production is untracked but NOT excluded by any applicable .gitignore, and contains secret-looking value(s) for: SECRET_TOKEN (a `git add .` away from being leaked)

2 finding(s)
```

JSON:

```
$ dotenvleak scan . --json
[
  {
    "rule": "DL001",
    "severity": "blocker",
    "path": ".env",
    "message": ".env is tracked by git and contains secret-looking value(s) for: API_KEY",
    "secretKeys": ["API_KEY"]
  }
]
```

## Pre-commit hook

Add to `.git/hooks/pre-commit` (or wire into a framework like
[pre-commit](https://pre-commit.com/)):

```sh
#!/bin/sh
dotenvleak scan . || {
  echo "dotenvleak found a blocking issue — see above." >&2
  exit 1
}
```

## CI example (GitHub Actions)

```yaml
- name: Check for leaked .env files
  run: |
    go install github.com/jay-tank/dotenvleak@latest
    dotenvleak scan .
```

## What this does NOT do

- It does not scan git **history** for secrets already committed in past
  commits — use gitleaks or truffleHog for that (see the root
  [README.md](../README.md) and [DETAILS.md](../DETAILS.md) for the exact
  differentiation).
- It does not verify whether a detected value is a live, working
  credential (unlike truffleHog's verification feature) — it only judges
  whether a value *looks* like a real secret vs. an obvious placeholder.
- It does not scan file types other than `.env*`-named files.
