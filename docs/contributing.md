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

These are one-time, human-driven steps. They are intentionally not automated — the credentials needed (admin token, app installations, PAT) are out of scope for the repo.

## Auto-merge

PRs labeled `automerge` are picked up by Kodiak once CI is green and approvals are satisfied. Renovate adds the label automatically; humans can add it manually for routine PRs.

## Regenerating sqlc queries

`internal/state/db/` is generated from `internal/state/schema.sql` and `internal/state/queries.sql` by [sqlc](https://sqlc.dev/). The generated code is committed; CI does not run sqlc. To regenerate after editing schema or queries:

```sh
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
cd internal/state && sqlc generate
```

Code review catches the case where queries change but the generated files don't.
