<div align="center">
  <img src="build/appicon.png" width="112" alt="Codex Claude Shuttle app icon">
  <h1>Codex Claude Shuttle</h1>
  <p><strong>Move local conversations across computers or between OpenAI Codex and Anthropic Claude Code.</strong></p>
  <p>No cloud service · No server deployment · No product account · No CLI for everyday use</p>
  <p>
    <a href="https://github.com/giraffegzy-bot/codex-claude-shuttle/actions/workflows/ci.yml"><img src="https://github.com/giraffegzy-bot/codex-claude-shuttle/actions/workflows/ci.yml/badge.svg" alt="CI status"></a>
    <a href="LICENSE"><img src="https://img.shields.io/github/license/giraffegzy-bot/codex-claude-shuttle" alt="MIT License"></a>
  </p>
  <p><strong>English</strong> · <a href="README.zh-CN.md">简体中文</a></p>
</div>

> [!IMPORTANT]
> Codex Claude Shuttle is an unofficial, independent open-source project. It is not affiliated with, endorsed by, or sponsored by OpenAI or Anthropic.

![Codex Claude Shuttle export and import demo](docs/media/demo.gif)

## Download

`v0.1.2` is an early public preview. Back up your current conversation data and test with non-critical conversations first.

| Platform | Preview download | Current status |
| --- | --- | --- |
| macOS Apple silicon | [App ZIP](https://github.com/giraffegzy-bot/codex-claude-shuttle/releases/download/v0.1.2/Codex-Claude-Shuttle-macOS-arm64.zip) | Self-signed; not Apple-notarized |
| Windows x64 | [Setup EXE](https://github.com/giraffegzy-bot/codex-claude-shuttle/releases/download/v0.1.2/Codex-Claude-Shuttle-Windows-x64-Setup.exe) | Unsigned; CI-built, physical-device acceptance pending |

Download [SHA256SUMS.txt](https://github.com/giraffegzy-bot/codex-claude-shuttle/releases/download/v0.1.2/SHA256SUMS.txt) with the installer and verify it before opening the app. See the [v0.1.2 release notes](https://github.com/giraffegzy-bot/codex-claude-shuttle/releases/tag/v0.1.2) for the complete preview boundary.

Intel Mac and Linux builds are not available. macOS or Windows may show an unknown-developer warning because the preview packages do not yet use commercial code signing.

## What It Does

Codex Claude Shuttle is for people who want to move selected local conversation history to another computer, or continue the visible part of a conversation in the other coding tool.

- Browse active Codex or Claude Code conversations by project, search them, and preview messages before export.
- Create a local ZIP transfer package with a manifest and SHA-256 integrity checks.
- Restore validated native conversations after mapping source projects to existing destination folders.
- Convert visible conversation text between Codex and Claude Code after showing the conversion boundary and requiring confirmation.
- Block writes while the destination tool is running, check conflicts and disk space, back up replacements, and verify committed files.
- Process conversation data entirely on the current computer.

## Supported Transfers

| Source | Destination | Result |
| --- | --- | --- |
| Codex | Codex | Preserves the validated native conversation and updates only destination path fields |
| Claude Code | Claude Code | Preserves the validated native conversation and updates only destination path fields |
| Codex | Claude Code | Converts visible user messages and final assistant replies |
| Claude Code | Codex | Converts visible user messages and final assistant replies |

Cross-tool conversion does **not** transfer internal reasoning, tool calls or results, subagents, Memory, Skills, attachments, authentication, configuration, or runtime state.

## Screenshots

![Codex Claude Shuttle home screen](docs/media/home.png)

| Select and preview conversations | Review an import package |
| --- | --- |
| ![Export screen](docs/media/export.png) | ![Import review screen](docs/media/import.png) |

## Use It

1. Install the preview build on the source and destination computers.
2. On the source computer, open **Export conversations**, choose Codex or Claude Code, select conversations, and save the ZIP package locally.
3. Move the ZIP package through a channel you trust. Transfer packages are not encrypted.
4. On the destination computer, open **Import conversations**, inspect the package, and map each source project to an existing local project folder.
5. Fully quit the destination tool, run the preflight check, review the write plan, and confirm the import.

Codex Claude Shuttle moves conversations only. It does not copy project source code or create missing project folders. A Shuttle account is never required; Codex or Claude Code must already have local conversation data on the relevant computer.

## Privacy And Safety

Conversation data is not uploaded by Codex Claude Shuttle. The app has no hosted backend and treats every transfer package as untrusted input before proposing a write.

SHA-256 checks detect changes to package contents, but they do not prove who created a package. Packages are not encrypted in the current version, so store and send them as sensitive files.

The app does not write to Codex SQLite, Claude global configuration, credentials, or private indexes. See [SECURITY.md](SECURITY.md) and the [threat model](docs/threat-model.md) for the security boundary and private reporting process.

## Current Limitations

- Apple notarization and commercial code signing are not configured.
- Windows physical-device installation and migration acceptance are still pending.
- Automated write tests use temporary roots and synthetic conversations; real-data write acceptance has not been completed against a user's current `~/.codex` or `~/.claude` directory.
- Claude Code continuation after cross-device import still needs real-device acceptance across supported client versions.
- Package encryption, automatic project-folder creation, in-app updates, and Linux builds are not available.
- Codex and Claude Code use private local formats that may change. Keep backups and test non-critical conversations first.

## Build From Source

Requirements: Go 1.25.12, Wails 2.14.0, Node.js 22.12 or newer, and pnpm 10.33.0.

```bash
git clone https://github.com/giraffegzy-bot/codex-claude-shuttle.git
cd codex-claude-shuttle
go install github.com/wailsapp/wails/v2/cmd/wails@v2.14.0
pnpm --dir frontend install --frozen-lockfile
pnpm --dir frontend build
go test ./...
go vet ./...
pnpm --dir frontend check
pnpm --dir frontend test
wails build -clean
```

The built application is written to `build/bin/`. See [CONTRIBUTING.md](CONTRIBUTING.md) for development rules, [docs/architecture.md](docs/architecture.md) for the architecture, and [docs/adr/0001-desktop-stack.md](docs/adr/0001-desktop-stack.md) for the desktop stack decision.

## Contributing

Issues and focused pull requests are welcome. Use synthetic data for every reproduction and never attach real transfer packages, conversations, full local paths, credentials, tokens, or cookies. Read [CONTRIBUTING.md](CONTRIBUTING.md) before contributing; report vulnerabilities privately according to [SECURITY.md](SECURITY.md).

## License

Codex Claude Shuttle is available under the [MIT License](LICENSE).
