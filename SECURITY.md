# Security Policy

## Supported Versions

Codex Claude Shuttle is currently a source preview. No production-ready release is supported yet. Security fixes are applied to the latest commit on the default branch; older commits and unofficial binaries are not supported.

The current code supports local Codex and Claude Code discovery, transfer-package export, and preflighted same-tool or cross-tool import. Automated tests use synthetic temporary directories. Physical Windows acceptance, real-data write acceptance, and real Claude Code continuation acceptance are still pending.

Back up existing conversation data and test with non-critical conversations before using a source build.

## Reporting a Vulnerability

Use **Report a vulnerability** on the repository's Security page to submit a private GitHub security advisory. Do not disclose vulnerability details in a public Issue.

If private reporting is unavailable, open a public Issue containing no technical details and ask the maintainers to establish a private communication channel.

Reports and reproductions must use synthetic data. Never upload a real transfer package, conversation, log containing message text, full local path, source file, credential, token, cookie, key, or private configuration.

Please include:

- the affected commit or version;
- the operating system and architecture;
- the transfer route involved;
- a minimal synthetic reproduction;
- the expected security impact;
- any suggested mitigation.

Maintainers will acknowledge a complete report when it has been reviewed. Response and remediation timelines depend on severity and maintainer availability; no fixed service-level agreement is currently offered.

## Security Boundary

- Transfer packages and local Codex or Claude Code conversation files are treated as untrusted input.
- Package hashes provide integrity checking, not sender authenticity.
- Transfer packages are not encrypted in the current version.
- The app does not directly write Codex SQLite, Claude global configuration, credentials, or private indexes.
- Import requires integrity validation, a read-only plan, destination-tool process checks, explicit confirmation, backup, constrained writes, readback verification, and a recovery journal.
- Cross-tool conversion handles only visible user messages and final assistant text, with a 128 MB per-conversation limit. Same-tool native payloads are processed as streams.
- The project independently validates its own implementation and does not treat similar open-source tools or private-format assumptions as a security approval.

See [docs/threat-model.md](docs/threat-model.md) for detailed threats and controls.
