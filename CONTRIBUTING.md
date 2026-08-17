# Contributing to Codex Claude Shuttle

Thanks for helping improve Codex Claude Shuttle. This project handles local conversation data and writes to private tool formats, so changes need a clear safety boundary and focused verification.

## Before Opening an Issue

- Search existing Issues first.
- Use the provided bug or feature template.
- Reproduce problems with synthetic or non-critical conversations.
- Remove usernames, account identifiers, full local paths, source code, and message content from screenshots and logs.
- Never upload a real transfer package, credential, token, cookie, key, or private configuration file.

Security vulnerabilities must be reported privately according to [SECURITY.md](SECURITY.md), not through a public Issue.

## Development Setup

Requirements:

- Go 1.25.12
- Wails 2.14.0
- Node.js 22.12 or newer
- pnpm 10.33.0

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.14.0
pnpm --dir frontend install --frozen-lockfile
```

Run the desktop app in development mode with:

```bash
wails dev
```

## Pull Requests

Keep changes scoped and explain the user-visible behavior, security impact, and verification performed. Pull requests should:

- preserve compatibility with existing transfer-package identifiers unless a versioned migration is included;
- keep file-system access and security decisions in Go rather than the frontend;
- use bounded, streamed processing for conversation payloads;
- include tests for changed behavior and failure paths;
- use synthetic fixtures only;
- avoid unrelated formatting, dependency, or generated-file churn;
- update README, architecture, threat model, or release notes when their documented behavior changes.

Before submitting, run:

```bash
pnpm --dir frontend build
go test ./...
go vet ./...
pnpm --dir frontend check
pnpm --dir frontend test
wails build -clean
```

macOS validation does not count as physical Windows acceptance. Describe the platforms and environments you actually tested.

## License

By contributing, you agree that your contribution will be licensed under the repository's [MIT License](LICENSE).
