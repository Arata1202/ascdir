# Accessibility declaration configuration

Accessibility Nutrition Labels are managed under `accessibility`, keyed by Apple device family. Supported keys are `IPHONE`, `IPAD`, `APPLE_TV`, `APPLE_WATCH`, `MAC`, and `VISION`.

Each declaration supports these boolean fields:

- `supports_audio_descriptions`
- `supports_captions`
- `supports_dark_interface`
- `supports_differentiate_without_color_alone`
- `supports_larger_text`
- `supports_reduced_motion`
- `supports_sufficient_contrast`
- `supports_voice_control`
- `supports_voiceover`

The `published` boolean represents the desired publication state. Apple does not support unpublishing a declaration. Changing it from `false` to `true` therefore requires `--allow-irreversible`; changing it from `true` to `false` is rejected locally.

```yaml
accessibility:
  IPHONE:
    published: false
    supports_voiceover: true
    supports_larger_text: true
```

Omit a field to leave it unmanaged. If a configured device-family declaration does not exist, `push` creates it before optionally publishing it.
