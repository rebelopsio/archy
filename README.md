# archy

archy is an extensible AI assistant for managing a markdown vault from Neovim and the CLI. Built on the Claude Agent SDK, it orchestrates Linear, GitHub, Google Calendar, and other systems into structured notes — daily briefs, meeting prep, review queues — via composable Skills, MCP servers, and typed blocks.

## Installation

Pre-built binaries for Linux and macOS (amd64 and arm64) are attached to each [GitHub release](https://github.com/rebelopsio/archy/releases). Download the archive matching your platform, extract, and move the `archy` binary somewhere on your `PATH`.

```sh
# example for macOS arm64
curl -L https://github.com/rebelopsio/archy/releases/latest/download/archy_<VERSION>_darwin_arm64.tar.gz | tar xz
sudo mv archy /usr/local/bin/
```

The `<VERSION>` placeholder is intentional — substitute the actual version (e.g., `0.1.0`). A Homebrew tap will replace this section when it lands.

Verify with `archy version`.

## Dependencies

This project uses [Mend Renovate](https://www.mend.io/renovate/) for dependency updates. Install the Mend Renovate GitHub App on the repo to enable. The configuration lives in [`.github/renovate.json`](.github/renovate.json).

## Contributing

archy uses Conventional Commits and squash-merge PRs so [release-please](https://github.com/googleapis/release-please) can drive versioning and the changelog automatically. See [`docs/contributing.md`](docs/contributing.md) for the commit conventions and the one-time repository-setup checklist (Renovate, Kodiak, branch protection).
