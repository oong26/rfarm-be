# Git Workflow

**English** · [Bahasa Indonesia](git_workflow.id.md)

[← Back to Documentation](README.md)

---

## Branches

`main` is always deployable and protected. All work happens on short-lived branches
merged through a pull request.

| Prefix | Use for | Example |
|---|---|---|
| `feat/` | new behaviour | `feat/sales-export` |
| `fix/` | bug fix | `fix/duplicate-daily-record` |
| `refactor/` | no behaviour change | `refactor/sale-store-interface` |
| `docs/` | documentation only | `docs/api-naming` |
| `chore/` | tooling, deps, config | `chore/update-go-1.25` |
| `test/` | tests only | `test/sale-service-branches` |
| `hotfix/` | urgent production fix, branched from a tag | `hotfix/login-500` |

Rules:

- Lowercase, kebab-case, 2–5 words: `feat/sales-export`, not
  `feature/newFeatureForExport`.
- One branch = one intent. If you need "and" to describe it, split it.
- Delete the branch after merge.
- Rebase on `main` before opening a PR; do not merge `main` into a feature branch to
  resolve a conflict.

---

## Commits

[Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <summary>

<body — why, not what>

<footer — refs, breaking changes>
```

| Type | Use for |
|---|---|
| `feat` | a new capability visible to a client |
| `fix` | a bug fix |
| `refactor` | code change with no behaviour change |
| `perf` | performance improvement |
| `test` | adding or fixing tests only |
| `docs` | documentation only |
| `build` | build system, dependencies, Makefile |
| `ci` | CI configuration |
| `chore` | maintenance that fits nowhere else |
| `revert` | reverting a previous commit |

`scope` is the domain or area: `sales`, `auth`, `stores`, `docs`, `deps`.

```text
feat(sales): add paginated list endpoint

Add GET /api/v1/sales with page/pageSize, month and from/to filters, q
search and an allowlisted sort. Meta carries page, pageSize and total.

Refs: PROJ-142
```

```text
fix(daily-records): return 409 when the date already exists

A unique violation from PostgreSQL was surfacing as 500. The store now
maps 23505 to Conflict with the duplicate_date code.

Refs: PROJ-151
```

Rules:

- Subject: imperative, lowercase after the type, no trailing period, ≤ 72 chars.
- One logical change per commit. Tests, the OpenAPI change and docs for that change
  go in the **same** commit.
- The body explains **why**; the diff already shows what.
- Reference the ticket in the footer when one exists.
- Breaking change: `feat(api)!: …` plus a `BREAKING CHANGE:` footer explaining the
  migration.
- Never commit commented-out code, debug prints, or a half-finished feature behind
  no flag.

---

## Generated Files

Some files are produced by a tool but committed. They follow one rule: **the commit
that changes the input also changes the output**.

| Input | Output | Command |
|---|---|---|
| `db/queries/*.sql`, migrations | `internal/stores/generated/` | `make sqlc` |
| `api/openapi.yaml` | generated types/stubs | `make openapi` |

- Never hand-edit a generated file.
- Never commit a generated change without its input change, or an input change
  without the regenerated output.
- `make check` fails when the committed output does not match what the tool would
  produce.

---

## Pull Requests

### Before opening

```bash
git fetch origin
git rebase origin/main
make check            # fmt + lint + sqlc diff + openapi check + test
```

### Title and description

Title follows the commit convention: `feat(sales): add paginated list endpoint`.

```markdown
## What
One paragraph: what changed and why.

## How
The approach, in 2–4 bullets. Mention alternatives rejected, if any.

## Verification
- [ ] `make check` passes
- [ ] Unit tests added/updated
- [ ] Integration tests added/updated (if SQL changed)
- [ ] `api/openapi.yaml` updated and `make openapi` run (if a contract changed)
- [ ] `make sqlc` run and output committed (if a query changed)
- [ ] Docs updated, `.id.md` twin included
- [ ] Manual check: <what you curled and what you saw>

## Risk
Blast radius, rollout notes, feature flag, migration order.

Refs: PROJ-142
```

### Size and scope

| Size | Guidance |
|---|---|
| ≤ 200 changed lines | easy to review, preferred |
| 200–400 | acceptable, explain the scope in the description |
| > 400 | split it; a migration or a mechanical rename may be its own PR |

- One concern per PR. A refactor plus a behaviour change is two PRs.
- Keep generated files in the PR that caused them.
- Never mix formatting-only changes with logic changes — review cannot see the
  difference.
- A schema change and the code that uses it may ship together, but the migration must
  be safe for the previous binary (expand → migrate → contract).

---

## Review

### Author

- Self-review the diff before requesting a review; delete leftovers.
- Answer every comment — with a change or a reason.
- Do not resolve a reviewer's comment yourself unless they asked you to.
- Push fixes as new commits while the review is open; squash happens at merge.

### Reviewer

Check, in this order:

1. **Correctness** — does it do what the description says, including edge cases?
2. **Tenancy and money** — every query filters `farm_id` and `deleted_at IS NULL`;
   no `float64` touched an amount.
3. **Layer discipline** — business rule in the service, SQL in the store, generated
   code untouched ([Architecture](architecture.md)).
4. **Error contract** — the right status and code (`412` vs `409`), no leaked
   internals, Indonesian message.
5. **Security** — role enforced on the new route, no IDOR, no secret in the diff.
6. **Tests** — the branch that matters is covered, and the test would fail if the code
   were reverted.
7. **Docs and contract** — `api/openapi.yaml` and the doc updated in the same commit,
   `.id.md` included.
8. **Readability** — names follow the standards, no dead code.

A review is not a rewrite. Block on correctness, security and contract; suggest the
rest.

---

## Merge

- **Squash and merge** is the default: one commit on `main` per PR, using the PR title
  as the subject and the description as the body.
- The squashed subject must follow Conventional Commits.
- Merge only when: CI is green, at least one approval, conversations resolved, and the
  branch is up to date.
- Delete the branch after merge.
- `main` is never force-pushed.

---

## Releases

- Tags follow semantic versioning: `v1.4.0`.
- A release is cut from `main` after CI is green.
- The release notes list user-visible changes (`feat`, `fix`, `perf`) and any breaking
  change with its migration steps.
- Database migrations ship with the code that needs them; apply before deploying the
  new binary, and keep the previous binary compatible for one release
  (expand → migrate → contract).
- CI publishes the Docker image and runs migrations as a separate, explicit step — not
  on application start.

---

## Hotfix

1. Branch from the tag currently in production: `hotfix/login-500`.
2. Fix, test, PR into `main` — small and focused.
3. Merge, tag a patch release (`v1.4.1`), deploy.
4. Cherry-pick into any maintained release branch if one exists.
5. Add a regression test in the same PR.

---

## Never Commit

| Never | Why |
|---|---|
| `.env`, `config.yaml`, secrets, tokens, certificates | leaked forever in history |
| build binaries, `bin/`, `tmp/`, coverage files | noise, platform-specific |
| editor and OS files (`.idea/`, `.vscode/`, `.DS_Store`) | personal config |
| commented-out code, `fmt.Println`, `TODO` without owner | dead weight |
| a migration edited after it ran | breaks every environment that applied it |
| hand-edited files under `internal/stores/generated` | the next `make sqlc` overwrites them |

Use `git add -p` and review the staged diff before every commit.

---

## Quick Reference

| Question | Answer |
|---|---|
| Base branch | `main` |
| Branch name | `feat/sales-export` |
| Commit format | `feat(sales): add export endpoint` |
| Breaking change | `feat(api)!:` + `BREAKING CHANGE:` footer |
| PR size | ≤ 400 lines, one concern |
| Merge method | squash and merge |
| Version tag | `vMAJOR.MINOR.PATCH` |
| Generated files | same commit as their input |
| Rebase or merge `main` in? | rebase on `main` |
| Delete branch after merge? | yes |
| Force-push `main`? | never |
