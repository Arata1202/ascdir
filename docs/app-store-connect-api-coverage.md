# App Store Connect API coverage

ascdir uses only documented, public App Store Connect API operations. It does
not automate App Store Connect web pages or depend on private endpoints.

The following product-page settings are visible in App Store Connect but are
not writable through Apple's public App Store Connect OpenAPI specification:

- App Privacy data-use declarations
- app tax category
- Apple Business Manager and Apple School Manager distribution settings
- last-compatible-version settings for existing customers

These settings remain intentionally unsupported. Adding configuration keys
without a corresponding API operation would create a false impression that
`push` can enforce them, while browser automation or private APIs would be
fragile and inappropriate for a general-purpose OSS CLI.

Support can be added when Apple publishes read and write operations that allow
ascdir to provide all of the following:

1. read-only `pull --dry-run` and `push --dry-run` behavior;
2. deterministic diffing and validation;
3. least-privilege API access;
4. explicit confirmation for destructive, commercial, or irreversible changes;
5. integration tests based on the documented request and response schemas.

## Sources

- [App Store Connect API](https://developer.apple.com/documentation/appstoreconnectapi/)
- [App metadata](https://developer.apple.com/documentation/appstoreconnectapi/app-metadata)
- [App pricing and availability](https://developer.apple.com/help/app-store-connect/reference/pricing-and-availability/app-pricing-and-availability)

Coverage was checked against Apple's downloadable OpenAPI specification on
2026-08-18.
