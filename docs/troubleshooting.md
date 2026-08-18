# Troubleshooting

## `ascdir.yaml not found`

Run ascdir from the directory containing `ascdir.yaml`, or pass its path explicitly with `--config`.

## Credentials or the private key cannot be found

Run `ascdir auth login` again and provide the issuer ID, key ID, and absolute path to the `.p8` file. The private key should remain outside the project and must not be committed. Use `ascdir auth check` to verify the saved credentials with a read-only request.

## The App Store version is not editable

Apple restricts most version metadata changes to editable app-version states. Select an editable version in `app.version`, or wait until its App Store Connect state permits editing.

## A locale or required file is missing

Every asset locale must also appear under `localizations`. Run `ascdir check` to identify missing files, invalid paths, unsupported values, and common length or URL problems before contacting App Store Connect.

## A push requires an explicit confirmation flag

ascdir protects destructive, irreversible, and commercial changes. Review `ascdir push --dry-run`, then use the flag named in the error, such as `--allow-empty`, `--allow-asset-deletions`, `--allow-irreversible`, or `--allow-commercial-changes`.

## A push stopped after applying some changes

App Store Connect does not provide a transaction across resource types. Correct the reported error, rerun `ascdir push --dry-run` to inspect the remaining difference, and retry. ascdir re-fetches the remote state and does not intentionally repeat changes that already succeeded.

## Markdown syntax appears as text

Long-form files use a `.md` extension for convenient source review, but App Store Connect treats their contents as plain text. Do not rely on Markdown or HTML rendering in product-page content.
