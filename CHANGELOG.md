# Changelog

All notable changes to ascdir are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and releases use [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Store short localized values directly in version 2 YAML configurations while keeping long-form content in Markdown files
- Keep version 1 file-per-field configurations fully supported
- Preserve YAML comments and key order when pulling inline values
- Validate App Store keywords against Apple's 100-byte limit

## [1.0.0] - 2026-08-17

### Added

- Initialize metadata projects from an existing App Store version
- Pull, validate, diff, and push localized text metadata
- Protect remote fields from accidental clearing
- Validate App Store character limits and URLs locally
- Authenticate with short-lived App Store Connect API JWTs
- Save, verify, and remove local API credential configuration
- Generate shell completion for Bash, Zsh, Fish, and PowerShell
- Install verified release archives with the macOS and Linux installer
- Retry safe requests after rate limits and transient server failures

[Unreleased]: https://github.com/Arata1202/ascdir/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/Arata1202/ascdir/releases/tag/v1.0.0
