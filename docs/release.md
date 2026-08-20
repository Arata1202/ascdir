# App Store submission and release

ascdir automates the recurring App Store Connect steps after a build has been uploaded and processed. It does not archive, sign, notarize, or upload an app binary.

## Prerequisites

- Upload the build with Xcode, Transporter, or your build pipeline.
- Wait until App Store Connect reports the build as `VALID`.
- Complete the metadata, compliance, agreements, and review information required by Apple.
- Use an App Store Connect API key whose role permits version and submission management.

Keep the `.p8` private key outside the repository. ascdir sends short-lived signed JWTs to Apple and never uploads the private key.

## Submit a version

Set `app.platform` and `app.version` in `ascdir.yaml`, then preview the plan:

```sh
ascdir check
ascdir push --dry-run
ascdir app-store submit --build 42 --dry-run
```

If the configured App Store version does not exist, the first plan contains only version creation. Confirm it, synchronize metadata, and then plan submission again:

```sh
ascdir app-store submit --confirm 1.2.0
ascdir push --dry-run
ascdir push
ascdir app-store submit --build 42 --dry-run
```

Stopping after version creation prevents an empty version from being submitted before product-page metadata has been reviewed and pushed. For an existing version, the plan resolves the build and submission draft before mutation. It refuses expired or non-valid builds, ambiguous matches, incompatible states, unrelated draft items, and replacement of an already selected different build.

After reviewing the plan, repeat the configured version as the confirmation token:

```sh
ascdir app-store submit --build 42 --confirm 1.2.0
```

If `--build` is omitted, ascdir selects the newest valid, unexpired build for the configured platform and version. Pinning the build is recommended in CI.

Release behavior is selected at submission time:

```sh
# Wait for a separate manual release command after approval (default)
ascdir app-store submit --release-type MANUAL --confirm 1.2.0

# Release automatically after approval
ascdir app-store submit --release-type AFTER_APPROVAL --confirm 1.2.0

# Do not release before the specified instant
ascdir app-store submit \
  --release-type SCHEDULED \
  --earliest-release-date 2026-09-01T09:00:00+09:00 \
  --confirm 1.2.0
```

App Store Connect does not provide a transaction spanning version creation, build selection, and review submission. If a later request fails, rerun the dry run. ascdir reads the current state, reuses only a compatible editable Review Submission, and reports the remaining work. Execution revalidates the plan immediately before the first mutation and stops if App Store Connect changed.

## Release an approved version

For `MANUAL` releases, wait until the version reaches `PENDING_DEVELOPER_RELEASE`, then run:

```sh
ascdir app-store release --dry-run
ascdir app-store release --confirm 1.2.0
```

This makes the approved version available to customers and therefore always requires confirmation. Versions configured for automatic or scheduled release are rejected by this command. A version already released or processing for distribution produces an empty plan.

## CI safety

- Run the dry-run command as an ordinary pull-request check.
- Restrict execution credentials to protected deployment environments.
- Pin `--build` and require an environment approval before the confirmed command.
- Do not print credential files or JWTs in logs.
- Treat `--confirm` as an intent guard, not as authorization; App Store Connect permissions remain authoritative.
