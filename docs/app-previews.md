# App Previews

Enable App Preview management with a project-relative directory:

```yaml
assets:
  app_previews: assets/app-previews
  preview_frame_times:
    en-US/IPHONE_67/01-demo.mp4: "00:00:05"
```

Store MOV, MP4, or M4V files using this layout:

```text
assets/app-previews/
  en-US/
    IPHONE_67/
      01-demo.mp4
```

The locale must exist under `localizations`. The directory name is an App
Store Connect `PreviewType`, and lexicographic file-name order is the App Store
display order. Each set may contain at most three videos. Poster-frame
timecodes are optional and keyed by the path relative to the preview root.

ascdir rejects files that are not ISO base media containers, files larger
than Apple's 500 MB limit, invalid poster-frame timecodes, and frame-time
entries that do not identify a managed video. Codec, duration, dimensions,
and frame rate remain authoritative server-side checks because their allowed
combinations depend on Apple's current preview type specifications.

`pull` downloads videos and records poster-frame timecodes in YAML. `push`
uses App Store Connect's reservation, chunked upload, and commit workflow.
Changing only a poster frame updates the existing video without re-uploading
it. Replacing or deleting a video requires `--allow-asset-deletions`.

If `pull` would remove a stale local preview, review `pull --dry-run` and rerun
with `--allow-local-asset-deletions`.

After commit, ascdir waits for Apple's asynchronous `videoDeliveryState` to
become `COMPLETE`. A failed delivery stops the push before the new ordering is
applied or an existing preview is removed.
