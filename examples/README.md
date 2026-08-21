# Examples

These directories are illustrative file layouts, not real git repositories
(dotenvleak needs an actual `.git` to check tracked/untracked state and
walk `.gitignore` chains, so a static example directory can't fully
reproduce it — see the integration tests in the package root, which build
real throwaway git repos, for that).

- `leaky-repo/` — shows the two blocker shapes: a `.env` that would be
  tracked, and `.env.production` files nested under `services/`, one of
  which has no local `.gitignore` protecting it (`DL002`).
- `clean-repo/` — shows the safe shape: a `.gitignore` that actually
  matches `.env*`, plus a committed `.env.example` template with
  placeholder-only values.

To turn `leaky-repo/` into something you can actually scan:

```sh
cp -r examples/leaky-repo /tmp/leaky-demo
cd /tmp/leaky-demo
git init -q
git add .env
git commit -q -m "oops"
dotenvleak scan .
```

Expected output:

```
[DL001] BLOCKER: .env is tracked by git and contains secret-looking value(s) for: API_KEY
[DL002] BLOCKER: services/worker/.env.production is untracked but NOT excluded by any applicable .gitignore, and contains secret-looking value(s) for: SECRET_TOKEN (a `git add .` away from being leaked)

2 finding(s)
```

And for the clean example:

```sh
cp -r examples/clean-repo /tmp/clean-demo
cd /tmp/clean-demo
git init -q
git add .
git commit -q -m "init"
dotenvleak scan .
```

Expected output:

```
dotenvleak: no issues found
```
