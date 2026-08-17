# Codex Claude Shuttle

[简体中文](README.zh-CN.md)

An unofficial, local-first desktop app for exporting, importing, and transferring conversations between OpenAI Codex and Anthropic Claude Code.

No cloud service. No server deployment. No product account. No command line is required for normal use.

> Codex Claude Shuttle is an independent open-source project. It is not affiliated with, endorsed by, or sponsored by OpenAI or Anthropic.

## Status

Codex Claude Shuttle is currently a source preview. Prebuilt installers have not been published yet.

- macOS Apple silicon builds are locally self-signed and not Apple-notarized.
- Windows x64 builds pass CI, but physical-device installation and migration acceptance are still pending.
- Intel Mac and Linux packages are not available.

When the first public release is ready, downloads and SHA-256 checksums will appear on the repository's Releases page. Until then, build from source only if you are comfortable testing an early desktop application with synthetic or non-critical conversations.

## What It Does

- Reads active conversations shown in the Codex or Claude Code sidebar. Archived, deleted, orphaned, and unknown-state records are excluded.
- Groups conversations by project, with search, selection, and message preview before export.
- Creates a local ZIP transfer package with a manifest and SHA-256 integrity checks.
- Restores native conversations to the same tool after mapping source projects to existing folders on the destination computer.
- Converts visible conversation text between Codex and Claude Code after showing the conversion boundary and requiring confirmation.
- Blocks writes while the destination tool is running, checks conflicts and disk space, backs up replacements, and verifies committed files.
- Processes conversation data entirely on the current computer.

## Supported Transfers

| Source | Destination | Result |
| --- | --- | --- |
| Codex | Codex | Preserves the validated native conversation and updates only destination path fields |
| Claude Code | Claude Code | Preserves the validated native conversation and updates only destination path fields |
| Codex | Claude Code | Converts visible user messages and final assistant replies |
| Claude Code | Codex | Converts visible user messages and final assistant replies |

Cross-tool conversion does not transfer internal reasoning, tool calls or results, subagents, Memory, Skills, attachments, authentication, configuration, or runtime state.

## How It Works

1. On the source computer, open **Export conversations** and choose Codex or Claude Code.
2. Select the projects and conversations to include, review the package, and save it locally.
3. Move the ZIP package to the destination computer through a channel you trust.
4. Open **Import conversations**, inspect the package, map source projects to existing local folders, close the destination tool, and run the preflight check before importing.

Codex Claude Shuttle moves conversations only. It does not copy project source code or create missing project folders.

## Privacy And Safety

Conversation data is not uploaded by Codex Claude Shuttle. The app does not require a hosted backend or product account, and it treats every transfer package as untrusted input before proposing a write.

SHA-256 checks detect changes to package contents, but they do not prove who created a package. Packages are not encrypted in the current version, so store and send them as sensitive files.

The app does not write to Codex SQLite, Claude global configuration, credentials, or private indexes. See [SECURITY.md](SECURITY.md) and the [threat model](docs/threat-model.md) for the current security boundary and reporting process.

## Current Limitations

- Apple notarization and commercial code signing are not configured.
- Windows physical-device acceptance is still pending.
- Automated write tests use temporary roots and synthetic conversations; real-data write acceptance has not been completed against a user's current `~/.codex` or `~/.claude` directory.
- Claude Code continuation after cross-device import still needs real-device acceptance across supported client versions.
- Package encryption, automatic project-folder creation, in-app updates, and Linux builds are not available.
- Codex and Claude Code use private local formats that may change. Test non-critical conversations first and keep backups.

## Build From Source

Requirements: Go 1.25.12, Wails 2.14.0, Node.js 22.12 or newer, and pnpm 10.33.0.

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.14.0
pnpm --dir frontend install --frozen-lockfile
pnpm --dir frontend build
go test ./...
go vet ./...
pnpm --dir frontend check
pnpm --dir frontend test
wails build -clean
```

The built application is written to `build/bin/`. Architecture details are in [docs/architecture.md](docs/architecture.md), and the desktop stack decision is in [docs/adr/0001-desktop-stack.md](docs/adr/0001-desktop-stack.md).

## Contributing

Read [CONTRIBUTING.md](CONTRIBUTING.md) before opening an Issue or pull request. Use synthetic data for every reproduction. Never attach real transfer packages, conversations, full local paths, credentials, tokens, or cookies.

## License

Codex Claude Shuttle is available under the [MIT License](LICENSE).
