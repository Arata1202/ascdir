# TestFlight distribution

`ascdir testflight distribute` handles recurring distribution after a build is uploaded and processed. It does not upload builds or create beta groups and testers.

## Prerequisites

- Upload the build using Xcode, Transporter, or a build pipeline and wait for `VALID` processing state.
- Create the intended internal or external beta groups in App Store Connect.
- For external testing, complete the app's Beta App Review details, encryption declarations, and a description for every Beta App localization required by Apple.
- Use an App Store Connect API key allowed to manage TestFlight builds and groups.

## Plan and distribute

Repeat `--group` for every existing group. Names are matched exactly.

```sh
ascdir testflight distribute \
  --build 42 \
  --group "Internal Team" \
  --group "Public Beta" \
  --dry-run

ascdir testflight distribute \
  --build 42 \
  --group "Internal Team" \
  --group "Public Beta" \
  --confirm 1.2.0
```

Omit `--build` to select the newest valid, unexpired build for `app.platform` and `app.version`. Pinning it is recommended in CI.

For internal groups, ascdir only attaches the build. When at least one external group is requested, it also reads the build's Beta App Review state. `WAITING_FOR_REVIEW`, `IN_REVIEW`, and `APPROVED` are preserved; no submission is duplicated. A missing or rejected submission is submitted after the requested group attachments.

External groups require an `APP_STORE_ELIGIBLE` build. `INTERNAL_ONLY` builds remain valid for internal groups but are rejected before any mutation when an external group is requested.

The dry run sends only GET requests. Confirmed execution recomputes the same plan before its first mutation and stops if the build, groups, membership, or review state changed. If a later request fails, run the dry run again; completed attachments disappear from the new plan and only remaining operations are retried.

## Boundaries

ascdir deliberately does not create, rename, or delete groups and does not invite or remove testers. Manage those less frequent account-level operations in App Store Connect, then reference the resulting group by name.
