# Text metadata

ascdir stores short, single-line values directly in `ascdir.yaml` and long-form values in project-relative files. Generated long-form files use a `.md` extension to make review convenient on GitHub, but ascdir sends their contents to App Store Connect as plain text. Markdown and HTML syntax are not rendered on the App Store.

## Generated localization files

### `description.md`

The localized App Store product description. It is required, supports up to 4,000 characters, and should explain the app's features and functionality.

### `promotional_text.md`

Optional localized promotional text displayed above the description. It supports up to 170 characters and can be changed without submitting a new app version.

### `whats_new.md`

Localized release notes for the selected app version. This field is required for app updates and is not required for the first version of a new app.

### `privacy_policy.md`

The localized privacy policy text required for a tvOS app. New configurations include this file only when `app.platform` is `TV_OS`. iOS and macOS apps use `privacy_policy_url`; `privacy_choices_url` is optional. These fields are separate from the App Privacy data-collection questionnaire.

### `license_agreement.md`

Optional custom end-user license agreement text shared by the configured territories. Omit the entire `license_agreement` section to leave the remote agreement unmanaged. Apps without a custom agreement use Apple's standard EULA; removing an existing custom agreement is an explicit clearing operation. See [Custom license agreement](license-agreement.md).

## Managed and unmanaged fields

A field present in the configuration is managed by ascdir. Removing a key or file reference leaves the corresponding remote field unchanged. Keeping an explicitly empty managed value requests that the remote value be cleared and may require `ascdir push --allow-empty`.

Existing version 1 and version 2 configurations remain supported, including configurations that explicitly manage `privacy_policy_text` on any platform. The platform-specific behavior applies only to newly generated configurations.
