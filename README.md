# ascdir

`ascdir` manages App Store Connect text metadata as reviewable YAML and Markdown. It lets teams review localized store content in Git, pull the current values, validate them locally, and push only the fields that changed.

## Features

- Keep short values in self-documenting YAML and long-form content in Markdown
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

This creates `ascdir.yaml` with short metadata values and downloads long-form content under `metadata/`.

Edit the YAML or Markdown files, validate them, and preview the remote changes:

```sh
ascdir check
ascdir push --dry-run
```

Apply the changes:

```sh
ascdir push
```

To replace managed local values with the current App Store Connect values:

```sh
ascdir pull --dry-run
ascdir pull
```

## Configuration

```yaml
version: "2"
app:
  id: "123456789"
  bundle_id: com.example.myapp
  platform: IOS
  version: 1.2.0
metadata:
  copyright: "2026 Example, Inc." # Year and rights holder, for example: 2026 Example, Inc.
  accessibility_url: "https://example.com/accessibility" # Optional public HTTP(S) accessibility information page
  content_rights_declaration: DOES_NOT_USE_THIRD_PARTY_CONTENT # Whether the app uses third-party content
categories:
  primary_category: PRODUCTIVITY # Required top-level App Store category ID
  primary_subcategory_one: "" # Optional first Games or Stickers subcategory ID
  primary_subcategory_two: "" # Optional second Games or Stickers subcategory ID
  secondary_category: UTILITIES # Optional secondary top-level App Store category ID
  secondary_subcategory_one: "" # Optional first secondary Games or Stickers subcategory ID
  secondary_subcategory_two: "" # Optional second secondary Games or Stickers subcategory ID
age_rating:
  advertising: false # Boolean capability or content declaration
  kids_age_band: "" # Made for Kids: FIVE_AND_UNDER, SIX_TO_EIGHT, NINE_TO_ELEVEN, or empty
  violence_cartoon_or_fantasy: NONE # Content frequency declaration
  developer_age_rating_info_url: "" # Optional public HTTP(S) age rating information page
accessibility:
  IPHONE:
    published: false # Publishing a declaration cannot be undone
    supports_voiceover: true # The app supports VoiceOver
    supports_larger_text: true # The app supports larger text
license_agreement:
  file: metadata/license_agreement.md # Custom EULA text; omit this section to keep the current agreement unmanaged
  territories: [JPN, USA] # App Store territory IDs where the custom EULA applies
localizations:
  en-US:
    values:
      name: Example App # App Store display name, up to 30 characters
      subtitle: A concise summary # Short summary displayed below the name, up to 30 characters
      keywords: example,productivity # Comma-separated search keywords, up to 100 bytes
      support_url: https://example.com/support # Public HTTP(S) support page
      marketing_url: https://example.com # Optional public HTTP(S) marketing page
      privacy_policy_url: https://example.com/privacy # Public HTTP(S) privacy policy
      privacy_choices_url: "" # Optional public HTTP(S) privacy choices page
    files:
      description: metadata/en-US/description.md # Markdown file containing the long app description
      promotional_text: metadata/en-US/promotional_text.md # Markdown file containing promotional text
      whats_new: metadata/en-US/whats_new.md # Markdown file containing release notes
      privacy_policy_text: metadata/en-US/privacy_policy.md # Markdown file containing the tvOS privacy policy
```

Short, single-line values live under `values`, where the key and generated inline comment describe the expected input. Long-form values live in files referenced under `files`. Paths are relative to the directory containing `ascdir.yaml`.

Remove a key to leave that field unmanaged. An explicitly empty value remains managed and represents a request to clear the remote field; `push` still requires `--allow-empty` when the remote value is non-empty.

Version 1 configurations remain fully supported. Existing projects can continue using one file per field without modification; newly initialized projects use version 2. To migrate manually, move short values into `values`, keep long-form paths under `files`, and change `version` to `"2"`.

Unknown configuration keys are rejected so misspelled fields cannot be silently ignored. During `pull`, ascdir updates only the managed YAML value nodes, preserving user comments and key order. The configuration and metadata files are staged before any destination is replaced, and each replacement is atomic.

Supported platforms are `IOS`, `MAC_OS`, `TV_OS`, and `VISION_OS`.

## Managed metadata

Non-localized fields:

- Version copyright
- App accessibility URL
- Content rights declaration
- Primary and secondary categories
- Games and Stickers subcategories
- Age rating declaration, including Made for Kids
- Accessibility Nutrition Labels for each device family
- Custom end-user license agreement text and territories

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

Downloads configured fields into `ascdir.yaml` and the referenced Markdown files. Local edits are overwritten, so commit or review them first. Use `pull --dry-run` to preview the local differences without writing files.

### `ascdir push`

Validates and compares local YAML and Markdown with App Store Connect, then updates only changed resources. Use `--dry-run` to inspect changes without writing.

Clearing a non-empty remote field requires the explicit `--allow-empty` flag:

```sh
ascdir push --dry-run
ascdir push --allow-empty
```

Changing `age_rating.kids_age_band` may become irreversible after App Review, so it also requires explicit confirmation:

```sh
ascdir push --dry-run
ascdir push --allow-irreversible
```

See [Age rating configuration](docs/age-rating.md) for every supported declaration and enum value.

Publishing an Accessibility Nutrition Label also requires `--allow-irreversible`. See [Accessibility declaration configuration](docs/accessibility.md).

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
