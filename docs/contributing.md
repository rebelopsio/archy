# Contributing

## Conventional commits

Commits on `main` follow [Conventional Commits](https://www.conventionalcommits.org/). release-please reads them to compute version bumps and to generate the changelog, so the convention is load-bearing rather than cosmetic.

| Type | When |
| --- | --- |
| `feat(scope): ...` | New user-facing capability |
| `fix(scope): ...` | Bug fix |
| `chore: ...` | Maintenance that doesn't ship a behavioral change |
| `docs: ...` | Documentation only |
| `test: ...` | Test-only changes |
| `refactor: ...` | Code change that does not alter behavior |
| `ci: ...` | CI / release tooling |

Breaking changes use `!` after the type/scope (e.g., `feat(write)!: ...`) or a `BREAKING CHANGE:` footer.

PRs should be **squash-merged** so the resulting commit message on `main` follows the convention. Kodiak is configured to do this automatically; see below.

### How conventional commits are enforced

This repo uses squash-merge. The squash commit on `main` takes its message from the PR title — not from any commit on the feature branch. release-please reads commits on `main` to drive versioning and the changelog, so the PR title is the load-bearing input.

PR titles are validated by [`.github/workflows/pr-title.yml`](../.github/workflows/pr-title.yml) using [`amannn/action-semantic-pull-request`](https://github.com/amannn/action-semantic-pull-request). The check runs on PR open, reopen, edit, and push. If your PR title doesn't match the conventional-commit spec, the check fails and the PR is blocked from merging.

Per-commit messages on a feature branch are **not** validated. Use whatever message style you find readable during development; the squash will replace them with the PR title anyway.

Accepted types: `feat`, `fix`, `chore`, `docs`, `test`, `refactor`, `ci`, `perf`, `style`, `revert`, `build`. Scope is optional. See [Conventional Commits](https://www.conventionalcommits.org/) for the full grammar.

## Repository setup checklist

After cloning this repo or creating it fresh, configure the following once:

1. Install the [Mend Renovate GitHub App](https://github.com/apps/renovate).
2. Install the [Kodiak GitHub App](https://github.com/marketplace/kodiakhq).
3. Create a fine-grained Personal Access Token scoped to this repo with `Contents: Read & Write` and `Pull requests: Read & Write`, then add it as a repo secret named `RELEASE_PLEASE_TOKEN`. This is required for release-please's PRs to trigger CI — without it, the default `GITHUB_TOKEN` actor cannot trigger downstream workflows, so release-please PRs sit forever waiting on `build`/`test`/`golangci-lint` checks that never run.
4. After the first CI run lands on `main`, configure branch protection on `main`:
   - Require pull request reviews: 1 approval.
   - Require status checks to pass: `build`, `test`, `golangci-lint`.
   - Require branches to be up to date before merging.
   - Require conversation resolution before merging.
5. Confirm release-please's first PR appears after the next conventional commit lands on `main`. Merge it to bootstrap `v0.1.0`.
6. After the `pr-title` workflow runs successfully on at least one PR, add `Validate PR title` to the required status checks for `main` in branch protection.
7. Install pre-commit hooks (per-clone, one-time): `pip install pre-commit && pre-commit install`.

These are one-time, human-driven steps. They are intentionally not automated — the credentials needed (admin token, app installations, PAT) are out of scope for the repo.

## Pre-commit hooks

This repo uses [pre-commit](https://pre-commit.com) for fast local feedback on formatting and linting. Setup is one-time per clone:

```sh
pip install pre-commit          # or: pipx install pre-commit
pre-commit install
```

The configured hooks mirror the CI checks (file hygiene, gofmt, goimports, go vet, go mod tidy, golangci-lint). To run all of them against the entire repo:

```sh
pre-commit run --all-files
```

To bypass hooks for a single commit (use sparingly):

```sh
git commit --no-verify
```

CI also runs the hooks via the `pre-commit` job in [`.github/workflows/lint.yml`](../.github/workflows/lint.yml), so anything that slips past local hooks is caught server-side.

Conventional-commit format is **not** enforced by pre-commit — release-please reads commits on `main` arriving via squash-merge, and the squash commit's message is the PR title, not anything a local hook ever validates. PR titles are validated by the `pr-title` workflow (see "How conventional commits are enforced" above).

## Auto-merge

PRs labeled `automerge` are picked up by Kodiak once CI is green and approvals are satisfied. Renovate adds the label automatically; humans can add it manually for routine PRs.

## Regenerating sqlc queries

`internal/state/db/` is generated from `internal/state/schema.sql` and `internal/state/queries.sql` by [sqlc](https://sqlc.dev/). The generated code is committed; CI does not run sqlc. To regenerate after editing schema or queries:

```sh
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
cd internal/state && sqlc generate
```

Code review catches the case where queries change but the generated files don't.
