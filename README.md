# dotenvleak

A small, dependency-free Go CLI that answers one narrow, high-value
question: **is a real `.env` file about to be (or already) leaked into this
git repository, right now, in the current working tree?**

`dotenvleak` scans a git repository for `.env*`-style files that look like
they contain real secrets, then checks two things about each one:

1. Is it **currently tracked** by git? If so, the secret is already in the
   repository (and in remote history the moment someone pushes).
2. If it's untracked, does **any applicable `.gitignore`** actually match
   it? If not, it's one `git add .` away from being committed — a
   near-miss waiting to happen.

It also flags a `.gitignore` that merely *looks* like it handles `.env`
files (e.g. a stray comment mentioning "env") but doesn't actually contain a
pattern that matches the filename, by doing real glob-pattern matching
instead of a literal string search.

## Why this, when gitleaks/truffleHog already exist?

They solve a different, complementary problem. See [DETAILS.md](DETAILS.md)
for the full comparison, but in short:

- **gitleaks / truffleHog**: scan git **history** for secrets that have
  already been committed, anywhere in the commit log. Reactive — tells you
  what has already leaked.
- **dotenvleak**: checks the **current working tree, right now** — is a
  `.env` file tracked today, and would the current `.gitignore` actually
  stop it from being tracked tomorrow. Proactive and pre-commit-friendly —
  tells you whether this specific, extremely common mistake is currently
  possible.

Neither tool replaces the other. Run history scanners for what's already
out there; run `dotenvleak` in CI / pre-commit for what's about to happen.

## Install

```sh
go install github.com/jay-tank/dotenvleak@latest
```

Or build from source:

```sh
CGO_ENABLED=0 go build -o dotenvleak .
```

## Usage

```sh
dotenvleak scan <repo-dir> [--json] [--strict] [--template-suffix=.ext] [--exclude-dir=name]
```

```sh
$ dotenvleak scan .
[DL001] BLOCKER: .env is tracked by git and contains secret-looking value(s) for: API_KEY
[DL002] BLOCKER: services/worker/.env.production is untracked but NOT excluded by any applicable .gitignore, and contains secret-looking value(s) for: SECRET_TOKEN (a `git add .` away from being leaked)

2 finding(s)
```

Exit codes: `0` clean, `1` at least one blocker (`DL001`/`DL002`, or `DL003`
under `--strict`), `2` usage error (not a git repo, bad path, bad flag).

See [docs/USAGE.md](docs/USAGE.md) for full flag documentation, CI and
pre-commit hook examples, and [DETAILS.md](DETAILS.md) for exactly how the
secret-detection heuristic and the `.gitignore` pattern matcher work, and
their documented limits.

## License

MIT, see [LICENSE](LICENSE).
