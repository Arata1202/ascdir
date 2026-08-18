# Custom license agreement

Add `license_agreement` to `ascdir.yaml` only when the app uses a custom EULA:

```yaml
license_agreement:
  file: metadata/license_agreement.md
  territories:
    - JPN
    - USA
```

The file contains the complete agreement text. Territory values are the
three-letter App Store territory IDs returned by the App Store Connect API.

Omitting the section leaves the remote setting unmanaged. Setting both the
file content and `territories` to empty removes the custom EULA and restores
Apple's standard agreement; as a clearing operation, this requires
`ascdir push --allow-empty`.
