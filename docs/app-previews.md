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

`pull` downloads videos and records poster-frame timecodes in YAML. `push`
uses App Store Connect's reservation, chunked upload, and commit workflow.
Changing only a poster frame updates the existing video without re-uploading
it. Replacing or deleting a video requires `--allow-asset-deletions`.

If `pull` would remove a stale local preview, review `pull --dry-run` and rerun
with `--allow-local-asset-deletions`.
