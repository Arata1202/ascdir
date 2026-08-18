package appstore

import "strconv"

var ageRatingFields = map[string]string{
	"advertising": "advertising",
	"alcohol_tobacco_or_drug_use_or_references": "alcoholTobaccoOrDrugUseOrReferences",
	"contests":                                         "contests",
	"gambling":                                         "gambling",
	"gambling_simulated":                               "gamblingSimulated",
	"guns_or_other_weapons":                            "gunsOrOtherWeapons",
	"health_or_wellness_topics":                        "healthOrWellnessTopics",
	"kids_age_band":                                    "kidsAgeBand",
	"loot_box":                                         "lootBox",
	"medical_or_treatment_information":                 "medicalOrTreatmentInformation",
	"messaging_and_chat":                               "messagingAndChat",
	"parental_controls":                                "parentalControls",
	"profanity_or_crude_humor":                         "profanityOrCrudeHumor",
	"age_assurance":                                    "ageAssurance",
	"sexual_content_graphic_and_nudity":                "sexualContentGraphicAndNudity",
	"sexual_content_or_nudity":                         "sexualContentOrNudity",
	"social_media":                                     "socialMedia",
	"social_media_age_restricted":                      "socialMediaAgeRestricted",
	"horror_or_fear_themes":                            "horrorOrFearThemes",
	"mature_or_suggestive_themes":                      "matureOrSuggestiveThemes",
	"unrestricted_web_access":                          "unrestrictedWebAccess",
	"user_generated_content":                           "userGeneratedContent",
	"violence_cartoon_or_fantasy":                      "violenceCartoonOrFantasy",
	"violence_realistic_prolonged_graphic_or_sadistic": "violenceRealisticProlongedGraphicOrSadistic",
	"violence_realistic":                               "violenceRealistic",
	"age_rating_override":                              "ageRatingOverrideV2",
	"korea_age_rating_override":                        "koreaAgeRatingOverride",
	"developer_age_rating_info_url":                    "developerAgeRatingInfoUrl",
}

var ageRatingBooleanFields = map[string]bool{
	"advertising": true, "gambling": true, "health_or_wellness_topics": true, "loot_box": true,
	"messaging_and_chat": true, "parental_controls": true, "age_assurance": true, "social_media": true,
	"social_media_age_restricted": true, "unrestricted_web_access": true, "user_generated_content": true,
}

func copyAgeRatingAttributes(target map[string]string, source map[string]any) {
	for local, remote := range ageRatingFields {
		switch value := source[remote].(type) {
		case string:
			target[local] = value
		case bool:
			target[local] = strconv.FormatBool(value)
		case nil:
			target[local] = ""
		}
	}
}
