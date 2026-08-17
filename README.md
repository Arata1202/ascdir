# ascdir

`ascdir` manages App Store Connect text metadata as local files. It lets teams review localized store content in Git, pull the current values, validate them locally, and push only the fields that changed.

## Features

- Store localized metadata in plain text and Markdown files
- Pull existing metadata from App Store Connect
- Preview exact changes before writing with `push --dry-run`
- Protect non-empty remote fields from accidental clearing
- Validate common App Store character limits and URLs locally
- Manage multiple locales from one `ascdir.yaml`
- Authenticate with App Store Connect API keys
- Follow paginated responses and retry rate limits and transient failures
- Run without telemetry or credential uploads

## Installation

With Homebrew on macOS or Linux:

```sh
brew install Arata1202/tap/ascdir
```

Download the archive for your platform from [GitHub Releases](https://github.com/Arata1202/ascdir/releases/latest), extract it, and place `ascdir` (or `ascdir.exe` on Windows) on your `PATH`.

On macOS or Linux, the checksum-verifying installer can do this automatically:

```sh
curl -fsSLO https://github.com/Arata1202/ascdir/releases/latest/download/install.sh
sh install.sh
rm install.sh
```

Verify the archive before extracting it:

```sh
# Linux
sha256sum --check checksums.txt --ignore-missing

# macOS
shasum -a 256 ascdir_*.tar.gz

# Windows PowerShell
Get-FileHash .\ascdir_*.zip -Algorithm SHA256
```

Compare the printed digest with the corresponding entry in `checksums.txt`.

Each release also includes a software bill of materials (SBOM) for its archives.

Alternatively, install from source with Go 1.26.6 or later:

```sh
go install github.com/Arata1202/ascdir/cmd/ascdir@latest
```

## Authentication

Create an App Store Connect API key, download its `.p8` private key once, and run:

```sh
ascdir auth login
ascdir auth check
```

The login command stores the issuer ID, key ID, and absolute path to the private key in your user configuration directory. The private key itself is not copied. For CI or temporary overrides, set:

```sh
export ASC_ISSUER_ID="00000000-0000-0000-0000-000000000000"
export ASC_KEY_ID="ABC123DEFG"
export ASC_PRIVATE_KEY_PATH="$HOME/.private_keys/AuthKey_ABC123DEFG.p8"
```

Never commit the private key. `ascdir` ignores `*.p8` and `.env` by default, but credentials should still be stored outside the repository or in a CI secret store.

The HTTP timeout defaults to 30 seconds. Set `ASCDIR_TIMEOUT` to a positive Go duration such as `45s` when needed.

Remove locally stored credentials without deleting the `.p8` private key:

```sh
ascdir auth logout
```

## Quick start

Initialize a project from an existing App Store version:

```sh
ascdir init \
  --bundle-id com.example.myapp \
  --platform IOS \
  --version 1.2.0
```

This creates `ascdir.yaml` and downloads the configured localization files under `metadata/`.

Edit the files, validate them, and preview the remote changes:

```sh
ascdir check
ascdir push --dry-run
```

Apply the changes:

```sh
ascdir push
```

To replace local files with the current App Store Connect values:

```sh
ascdir pull --dry-run
ascdir pull
```

## Configuration

```yaml
version: "1"
app:
  id: "123456789"
  bundle_id: com.example.myapp
  platform: IOS
  version: 1.2.0
localizations:
  en-US:
    name: metadata/en-US/name.txt
    subtitle: metadata/en-US/subtitle.txt
    description: metadata/en-US/description.md
    keywords: metadata/en-US/keywords.txt
    promotional_text: metadata/en-US/promotional_text.txt
    whats_new: metadata/en-US/whats_new.md
    support_url: metadata/en-US/support_url.txt
    marketing_url: metadata/en-US/marketing_url.txt
    privacy_policy_url: metadata/en-US/privacy_policy_url.txt
    privacy_choices_url: metadata/en-US/privacy_choices_url.txt
    privacy_policy_text: metadata/en-US/privacy_policy.md
```

Paths are relative to the directory containing `ascdir.yaml`. Set a field path to an empty string or remove it to leave that field unmanaged.

Unknown configuration keys are rejected so misspelled fields cannot be silently ignored. Metadata files are replaced atomically to avoid partially written files.

Supported platforms are `IOS`, `MAC_OS`, `TV_OS`, and `VISION_OS`.

## Managed metadata

App-level localization fields:

- Name and subtitle
- Privacy policy URL and text
- Privacy choices URL

Version-level localization fields:

- Description and keywords
- Promotional text and what's new text
- Support URL and marketing URL

When a configured locale does not exist remotely, `push` creates both the app-level and version-level localization resources in the order required by App Store Connect.

## Commands

### `ascdir auth check`

Validates the configured private key, creates a short-lived JWT locally, and performs a read-only API request.

### `ascdir auth login`

Prompts for the issuer ID, key ID, and `.p8` path, validates the private key, and saves the configuration with user-only file permissions. On Unix systems, ascdir warns when the private key is readable by other users.

### `ascdir auth logout`

Removes the credentials saved by `auth login`. It never deletes the `.p8` private key. Environment variables remain untouched.

### `ascdir init`

Finds an existing app and version, generates the configuration, and pulls its localizations. It refuses to overwrite an existing configuration unless `--force` is supplied.

### `ascdir pull`

Downloads configured fields. Local edits are overwritten, so commit or review them first. Use `pull --dry-run` to preview the local differences without writing files.

### `ascdir push`

Validates and compares local files with App Store Connect, then updates only changed resources. Use `--dry-run` to inspect changes without writing.

Clearing a non-empty remote field requires the explicit `--allow-empty` flag:

```sh
ascdir push --dry-run
ascdir push --allow-empty
```

Apple only permits most version metadata to change while the version is in an editable state. API errors are returned without hiding Apple's error code or detail.

### `ascdir check`

Checks the configuration, required files, common character limits, and HTTP(S) URLs without contacting App Store Connect.

### `ascdir completion`

Prints shell completion for Bash, Zsh, Fish, or PowerShell. For example, enable Zsh completion for the current session with:

```sh
source <(ascdir completion zsh)
```

## Scope

`ascdir` manages localized text metadata. It does not upload builds or screenshots, submit versions for review, manage TestFlight, certificates, subscriptions, analytics, or customer reviews.

## Security

- Authentication uses short-lived ES256 JSON Web Tokens generated locally.
- The `.p8` private key is read from the configured local path and never leaves the machine. Apple receives only the signed JWT.
- API errors are reported without printing credentials or JWTs.
- `push --dry-run` never sends mutation requests.
- Pagination links are restricted to the configured App Store Connect API origin, preventing bearer tokens from being forwarded to another host.
- Metadata paths are confined to the configuration directory after resolving symbolic links.
- Retried mutations are limited to idempotent updates and requests rejected by rate limiting.
- ascdir collects no telemetry.

## Development

```sh
make fmt
make check
make build
```

Tests use generated keys and local HTTP servers. They never require real App Store Connect credentials or contact the production API.

See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution guidelines, [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) for community expectations, and [SECURITY.md](SECURITY.md) for private vulnerability reporting.

## License

MIT

This is an independent, unofficial project and is not affiliated with, endorsed by, or sponsored by Apple Inc. App Store Connect is a trademark of Apple Inc.
