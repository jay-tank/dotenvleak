# DETAILS

## The gap this fills, honestly

Leaked `.env` files are one of the most common real-world secret-leak
vectors on GitHub; GitGuardian's annual State of Secrets Sprawl reports
call out `.env`-style credential files repeatedly as a recurring, high-volume
leak category. The failure mode is almost always the same: a developer
runs `git add .` (or `git add -A`) in a repo whose `.gitignore` doesn't
actually exclude their local `.env`, and it gets committed and pushed
before anyone notices.

**gitleaks** and **truffleHog** are the established tools here, and this
package does not try to replace either of them. It fills a narrower,
complementary gap:

| | scans | when | answers |
|---|---|---|---|
| **gitleaks / truffleHog** | git **history** (every commit, ever) | after the fact | "has a secret already been committed anywhere in this repo's history?" (reactive) |
| **dotenvleak** | the **current working tree** only | before/at commit time | "is a real `.env` file tracked right now, or would it slip past `.gitignore` if added right now?" (proactive) |

Concretely, per current (2026) comparisons of the two established tools:
Gitleaks is commonly run as a fast pre-commit hook and in CI to catch
secrets in diffs and history, while TruffleHog additionally verifies
whether a found credential is still live and can scan beyond git (S3,
Docker images, etc.) — both are still fundamentally scanning *content that
already exists as committed data* (working tree, staged diff, or full
history), using secret-shaped regex/entropy detectors across arbitrary file
types.

`dotenvleak` does something those tools don't specifically do: it asks
git two narrow, structural questions about `.env*` files specifically —
"is this path in `git ls-files` right now" and "does a real `.gitignore`
pattern in the applicable chain actually match this filename" — rather
than scanning file *content* for secret-shaped strings across history. A
repo can be perfectly clean by gitleaks' history-scan standards (nothing
has been committed yet) and still be one `git add .` away from a leak
because its `.gitignore` has a typo or scoping bug. That "about to happen"
window is exactly what `dotenvleak` targets. It is meant to run *alongside*
gitleaks/truffleHog (pre-commit and in CI), not instead of them — and it is
useless for the case they exist for: finding a secret already buried three
years deep in commit history. Use gitleaks/truffleHog for that; nothing
here attempts it.

## Why this tool shells out to `git` (the one deliberate exception)

Every other tool in this workspace's package lineup is pure static
parsing with no subprocess calls. `dotenvleak` is a deliberate, documented
exception: "is this file tracked" and "does `.gitignore` protect it" are
concepts git itself defines and owns (including precedence rules across
`.gitignore`, `.git/info/exclude`, a configured global excludesfile,
sparse-checkout, assume-unchanged bits, etc). Re-implementing git's
tracking database and its full ignore-resolution engine from scratch would
be a large, bug-prone undertaking that would still just be a worse answer
than asking git directly. `os/exec` calls to `git rev-parse`, `git
ls-files --error-unmatch`, are used for exactly the tracked/not-tracked
question, where git's own answer is authoritative. Everything else
(finding candidate files, parsing `KEY=value` content, matching
`.gitignore` glob patterns against a filename) is done with dotenvleak's
own code, documented below.

## The `.gitignore` pattern matcher's exact scope

`dotenvleak` implements a pragmatic, from-scratch subset of gitignore glob
syntax (see `gitignore.go`) rather than depending on a third-party library,
so its exact behavior is fully auditable in one small file. It walks from
the repository root down to the candidate file's directory, collecting (in
that order) `.git/info/exclude` and every `.gitignore` found along the way,
then evaluates patterns in that order — root-first, in-file order
preserved — applying git's real "last matching pattern wins" rule so that a
more specific/deeper `.gitignore` can override a broader one higher up the
tree, and a later negation (`!pattern`) can re-include a path an earlier
pattern excluded.

**Supported:**
- comments (`#`) and blank lines
- `!` negation
- a leading `/` anchoring the pattern to the `.gitignore`'s own directory
- a pattern containing any other `/` is anchored the same way (matches
  git's real "contains a slash → anchored" rule)
- a pattern with **no** `/` at all matches the file's basename at any depth
  below that `.gitignore`'s directory
- `*` and `?` glob semantics within one path segment (via Go's
  `path.Match`)
- `**` as a whole segment, matching zero or more path segments, at the
  start (`**/x`), end (`x/**`), or middle (`a/**/b`) of a pattern
- trailing `/` (directory-only patterns) — accepted and parsed, though
  since dotenvleak only ever tests plain files against patterns, this only
  affects the recorded pattern metadata, not matching behavior

**Explicitly NOT supported** (and why this is an acceptable, documented
trade-off rather than a silent gap):
- POSIX character classes (`[aA]bc`, `[!ab]`) are treated as literal
  characters rather than expanded. These are rare in real-world
  `.gitignore` files protecting `.env` paths specifically, and mishandling
  them silently would be worse than declining to expand them.
- Backslash-escaping of a leading `!` or `#` (e.g. `\!important`) is not
  implemented; such lines are treated using the unescaped meaning.
- Exotic edge cases around trailing-whitespace escaping are not
  implemented beyond a simple trim of unescaped trailing spaces/tabs.

This means `dotenvleak`'s gitignore check can, in rare cases involving
character classes or escaped special characters, disagree with git's own
`git check-ignore`. For the overwhelmingly common real-world patterns used
to protect `.env` files (`*.env`, `.env*`, `/.env`, `.env.local`,
`**/.env`, etc.) it is accurate.

## The secret-detection heuristic, precisely

For every `.env*` candidate file found (see filename rules below),
`dotenvleak` parses `KEY=value` lines (`secrets.go`: `parseEnvAssignments`)
skipping blank lines and `#` comments, stripping an optional leading
`export `, requiring the key to look like a shell/env identifier
(`[A-Za-z_][A-Za-z0-9_]*`), and unquoting a single- or double-quoted value.
This is intentionally simple dotenv parsing — it does not handle multi-line
quoted values or line continuations. Real-world `.env` files are
overwhelmingly single-line `KEY=value` pairs, so this is a pragmatic
scope, not a shortcut that silently drops common cases.

A key is considered **secret-shaped** if it contains (case-insensitive) any
of: `PASSWORD`, `SECRET`, `TOKEN`, `KEY`, `API_KEY`, `PRIVATE`,
`CREDENTIAL`. This is a deliberately high-recall, lower-precision
heuristic: something like `SECRET_LOG_LEVEL` or `PUBLIC_KEY_ID` (not
actually secret) will be flagged (false positive) in exchange for rarely
missing an actual credential (false negative). Given the cost asymmetry —
a human glancing at a flagged non-secret line costs seconds, a missed real
secret costs an incident — this trade-off is deliberate.

A value is treated as an **obvious placeholder** (and therefore excluded
from "secret-looking") if it is: empty; wrapped in angle brackets
(`<your-secret-here>`); an exact case-insensitive match against a small
known list (`xxx`, `changeme`, `change_me`, `change-me`, `your-key-here`,
`placeholder`, `todo`, `tbd`, `n/a`, `na`); or composed entirely of
repeated `x` characters (`xxx`, `xxxxxxxx`). This is intentionally a
**precise, low-recall** heuristic in the other direction: it only
recognizes conventional placeholder idioms rather than guessing at
arbitrary "looks fake" strings, because a broader guess starts producing
false negatives (treating a genuinely truncated/rotated-out real secret as
a "placeholder" and staying silent) that are worse than the noise of a
slightly too-permissive definition of "real-looking."

Net effect: `dotenvleak` will have false positives (flagging a non-secret
whose key name happens to contain "KEY" or "TOKEN") and rare false
negatives (a genuinely secret value that happens to match the narrow
placeholder list, or a key name that doesn't contain any of the listed
markers, e.g. `STRIPE_SK` without "KEY/SECRET/TOKEN" in it). This is
documented, not hidden: `dotenvleak` is a heuristic triage tool, not a
guarantee. `--strict` (surfacing DL003, "a `.env*` file exists with no
clearly secret-looking content detected") exists specifically to catch the
lower-confidence tail for teams that want maximum recall over noise.

## Template-file exclusion

`.env.example`, `.env.sample`, `.env.template`, and `.env.dist` (suffix
match, case-insensitive) are excluded from scanning entirely by default,
because these are meant to be committed: they document the shape of the
environment configuration an application expects, populated with
placeholder values, specifically so a developer can `cp .env.example .env`
locally and never commit the real one. Flagging them would be a false
positive against an intentional, safe, extremely common pattern. The
suffix list is configurable via repeated `--template-suffix=` flags on the
CLI (see `docs/USAGE.md`), for teams with a different naming convention
(e.g. `.env.default`).

## Findings reference

- **DL001** (blocker) — a `.env*` file with secret-looking content **is
  currently tracked** by git. The secret is already committed.
- **DL002** (blocker) — a `.env*` file with secret-looking content is
  **untracked**, but no `.gitignore` pattern in the applicable chain would
  actually exclude it. A near-miss: one `git add .` away from DL001.
- **DL003** (warning, opt-in via `--strict`) — a `.env*` file exists
  (tracked or not) with no clearly secret-looking content detected by the
  heuristic above. Off by default to avoid noise on legitimately empty or
  placeholder-only `.env` files; still worth a human glance since the
  content heuristic is not exhaustive.

## Sources consulted

- GitGuardian's annual "State of Secrets Sprawl" reports repeatedly
  identify `.env`/credential files as a recurring high-volume leak
  category found in public repositories.
- Current (2026) comparisons of Gitleaks and TruffleHog describe both as
  scanning committed git content (history, diffs, and in TruffleHog's
  case, live credential verification and non-git sources) — i.e. reactive,
  content-scanning tools — which is the basis for the "history-scan vs.
  current-state proactive check" framing above.
