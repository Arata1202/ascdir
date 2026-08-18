# Age rating configuration

`age_rating` maps directly to the App Store Connect age rating declaration. `ascdir init` writes every value returned by Apple. In an existing configuration, omitted keys remain unmanaged.

Boolean declarations accept `true` or `false`:

- `advertising`
- `gambling`
- `health_or_wellness_topics`
- `loot_box`
- `messaging_and_chat`
- `parental_controls`
- `age_assurance`
- `social_media`
- `social_media_age_restricted`
- `unrestricted_web_access`
- `user_generated_content`

Content-frequency declarations accept `NONE`, `INFREQUENT_OR_MILD`, `FREQUENT_OR_INTENSE`, `INFREQUENT`, or `FREQUENT`:

- `alcohol_tobacco_or_drug_use_or_references`
- `contests`
- `gambling_simulated`
- `guns_or_other_weapons`
- `medical_or_treatment_information`
- `profanity_or_crude_humor`
- `sexual_content_graphic_and_nudity`
- `sexual_content_or_nudity`
- `horror_or_fear_themes`
- `mature_or_suggestive_themes`
- `violence_cartoon_or_fantasy`
- `violence_realistic_prolonged_graphic_or_sadistic`
- `violence_realistic`

Other declarations:

- `kids_age_band`: empty, `FIVE_AND_UNDER`, `SIX_TO_EIGHT`, or `NINE_TO_ELEVEN`
- `age_rating_override`: empty, `NONE`, `NINE_PLUS`, `THIRTEEN_PLUS`, `SIXTEEN_PLUS`, `EIGHTEEN_PLUS`, or `UNRATED`
- `korea_age_rating_override`: empty, `NONE`, `FIFTEEN_PLUS`, or `NINETEEN_PLUS`
- `developer_age_rating_info_url`: empty or a public HTTP(S) URL

Apple treats `kids_age_band` as the Made for Kids declaration. Because changing it may become irreversible after App Review, a non-dry-run push containing that field requires `--allow-irreversible`.

Always inspect the complete change set first:

```sh
ascdir push --dry-run
ascdir push --allow-irreversible
```
