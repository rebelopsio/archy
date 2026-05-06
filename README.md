# archy

archy is an extensible AI assistant for managing a markdown vault from Neovim and the CLI. Built on the Claude Agent SDK, it orchestrates Linear, GitHub, Google Calendar, and other systems into structured notes — daily briefs, meeting prep, review queues — via composable Skills, MCP servers, and typed blocks.

## Dependencies

This project uses [Mend Renovate](https://www.mend.io/renovate/) for dependency updates. Install the Mend Renovate GitHub App on the repo to enable. The configuration lives in [`.github/renovate.json`](.github/renovate.json).

## Contributing

archy uses Conventional Commits and squash-merge PRs so [release-please](https://github.com/googleapis/release-please) can drive versioning and the changelog automatically. See [`docs/contributing.md`](docs/contributing.md) for the commit conventions and the one-time repository-setup checklist (Renovate, Kodiak, branch protection).
