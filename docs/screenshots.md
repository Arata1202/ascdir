# Screenshots

Enable screenshot management with one project-relative directory:

```yaml
assets:
  screenshots: assets/screenshots
```

Store PNG or JPEG files using this layout:

```text
assets/screenshots/
  en-US/
    APP_IPHONE_67/
      01-home.png
      02-details.png
```

The locale must exist under `localizations`. The directory name is an App
Store Connect `ScreenshotDisplayType`, and lexicographic file-name order is the
App Store display order. A set may contain at most 10 images.

`pull` downloads the current remote images and removes stale PNG/JPEG files
inside the managed directory. `push` compares source checksums, uploads new
files in the chunks requested by App Store Connect, commits each upload, and
then updates the display order. Existing matching assets are reused.

When stale local screenshots would be removed, `pull` stops until the plan has
been reviewed with `pull --dry-run` and explicitly confirmed with
`--allow-local-asset-deletions`.

After commit, ascdir waits for Apple's asynchronous `assetDeliveryState` to
become `COMPLETE`. A failed delivery stops the push before the new ordering is
applied or an existing screenshot is removed.

Replacing or deleting a remote image requires both a dry-run review and the
explicit `--allow-asset-deletions` option:

```sh
ascdir push --dry-run
ascdir push --allow-asset-deletions
```
