# Changelog

All notable changes to ascdir are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and releases use [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- Isolate credential tests from real user configuration directories on every supported OS.
- Keep pulled assets inside dedicated managed roots, reject unsafe filenames and symlinks, and prevent configuration or metadata path collisions.
- Reject missing asset directories instead of treating them as an intentional empty set, and require confirmation before pull removes local assets.
- Validate downloaded asset checksums and metadata UTF-8, clean up failed App Preview downloads, and roll back multi-file pull failures.
- Preserve nested YAML comments and support `init --config` paths whose parent directories do not exist yet.

## [1.1.3] - 2026-08-19

### Added

- Add a safe minimal configuration example for first-time setup.

### Changed

- Group empty managed fields by location after `init`, reduce repeated generated comments, and guide users through validation and the first dry run.

## [1.1.2] - 2026-08-19

### Fixed

- Generate `privacy_policy.md` only for newly initialized tvOS projects while preserving existing configurations that manage the field explicitly.

### Changed

- Clarify that long-form `.md` files are sent to App Store Connect as plain text.
- Document text metadata lifecycle, troubleshooting, supported release platforms, and additional installation methods.

## [1.1.1] - 2026-08-19

### Fixed

- Preserve local screenshot and App Preview file paths while associating new asset sets with their version localization.
- Create missing localization resources before uploading their product-page assets.
- Clean up reserved assets after failed uploads or ordering requests.
- Bound App Preview downloads to prevent unbounded temporary-file growth.

### Changed

- Document partial-application recovery when App Store Connect rejects a later request in a multi-resource push.

## [1.1.0] - 2026-08-19

### Added

- Manage version copyright and the app accessibility URL as optional YAML values.
- Manage the app content rights declaration as an optional YAML value.
- Manage primary, secondary, Games, and Stickers App Store categories in YAML.
- Manage the complete App Store age rating declaration, including Made for Kids, with explicit confirmation for potentially irreversible changes.
- Manage Accessibility Nutrition Label declarations per device family, including explicit publication safeguards.
- Manage custom end-user license agreement text and its applicable territories.
- Upload, order, replace, and remove localized App Store screenshots with explicit deletion safeguards.
- Upload, order, replace, and remove localized App Preview videos, including optional poster-frame timecodes, with explicit deletion safeguards.
- Manage territory availability, release dates, and preorder settings with explicit confirmation for availability changes.
- Look up App Store price points and manage base-territory pricing schedules with explicit confirmation for commercial changes.

### Changed

- Store short localized values directly in version 2 YAML configurations while keeping long-form content in Markdown files.
- Keep version 1 file-per-field configurations fully supported.
- Preserve YAML comments and key order when pulling inline values.
- Validate App Store keywords against Apple's 100-byte limit.

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

[Unreleased]: https://github.com/Arata1202/ascdir/compare/v1.1.3...HEAD
[1.1.3]: https://github.com/Arata1202/ascdir/compare/v1.1.2...v1.1.3
[1.1.2]: https://github.com/Arata1202/ascdir/compare/v1.1.1...v1.1.2
[1.1.1]: https://github.com/Arata1202/ascdir/compare/v1.1.0...v1.1.1
[1.1.0]: https://github.com/Arata1202/ascdir/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/Arata1202/ascdir/releases/tag/v1.0.0
