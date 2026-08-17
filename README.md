# ascdir

`ascdir` manages App Store Connect text metadata as local files. It lets teams review localized store content in Git, pull the current values, validate them locally, and push only the fields that changed.

## Features

- Store localized metadata in plain text and Markdown files
- Pull existing metadata from App Store Connect
- Preview exact changes before writing with `push --dry-run`
- Validate common App Store character limits and URLs locally
- Manage multiple locales from one `ascdir.yaml`
- Authenticate with App Store Connect API keys

## Installation

Build from source with Go 1.26 or later:

```sh
go install github.com/Arata1202/ascdir/cmd/ascdir@latest
```

## Authentication

Create an App Store Connect API key, download its `.p8` private key once, and set:

```sh
export ASC_ISSUER_ID="00000000-0000-0000-0000-000000000000"
export ASC_KEY_ID="ABC123DEFG"
export ASC_PRIVATE_KEY_PATH="$HOME/.private_keys/AuthKey_ABC123DEFG.p8"
```

Never commit the private key. `ascdir` ignores `*.p8` and `.env` by default, but credentials should still be stored outside the repository or in a CI secret store.

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
```

Paths are relative to the directory containing `ascdir.yaml`. Set a field path to an empty string or remove it to leave that field unmanaged.

Supported platforms are `IOS`, `MAC_OS`, `TV_OS`, and `VISION_OS`.

## Commands

### `ascdir init`

Finds an existing app and version, generates the configuration, and pulls its localizations. It refuses to overwrite an existing configuration unless `--force` is supplied.

### `ascdir pull`

Downloads configured fields. Local edits are overwritten, so commit or review them first.

### `ascdir push`

Compares local files with App Store Connect and updates only changed resources. Use `--dry-run` to inspect changes without writing.

### `ascdir check`

Checks the configuration, required files, common character limits, and HTTP(S) URLs without contacting App Store Connect.

## Scope

`ascdir` manages localized text metadata. It does not upload builds or screenshots, submit versions for review, manage TestFlight, certificates, subscriptions, analytics, or customer reviews.

## Security

- Authentication uses short-lived ES256 JSON Web Tokens generated locally.
- The `.p8` private key is read from the path in `ASC_PRIVATE_KEY_PATH` and is never sent anywhere except through the JWT signature required by Apple.
- API errors are reported without printing credentials or JWTs.
- `push --dry-run` never sends mutation requests.

## Development

```sh
make fmt
make check
make build
```

## License

MIT

This is an independent, unofficial project and is not affiliated with, endorsed by, or sponsored by Apple Inc. App Store Connect is a trademark of Apple Inc.
