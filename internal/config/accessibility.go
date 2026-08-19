package config

import (
	"sort"
	"strconv"
)

// AccessibilityValues is one Accessibility Nutrition Label declaration,
// keyed by Apple device family in Config.Accessibility.
type AccessibilityValues struct {
	Published                              *bool `yaml:"published,omitempty"`
	SupportsAudioDescriptions              *bool `yaml:"supports_audio_descriptions,omitempty"`
	SupportsCaptions                       *bool `yaml:"supports_captions,omitempty"`
	SupportsDarkInterface                  *bool `yaml:"supports_dark_interface,omitempty"`
	SupportsDifferentiateWithoutColorAlone *bool `yaml:"supports_differentiate_without_color_alone,omitempty"`
	SupportsLargerText                     *bool `yaml:"supports_larger_text,omitempty"`
	SupportsReducedMotion                  *bool `yaml:"supports_reduced_motion,omitempty"`
	SupportsSufficientContrast             *bool `yaml:"supports_sufficient_contrast,omitempty"`
	SupportsVoiceControl                   *bool `yaml:"supports_voice_control,omitempty"`
	SupportsVoiceover                      *bool `yaml:"supports_voiceover,omitempty"`
}

func (v AccessibilityValues) Map() map[string]string {
	result := map[string]string{}
	for field, pointer := range v.Pointers() {
		if pointer != nil {
			result[field] = *pointer
		}
	}
	return result
}

func (v AccessibilityValues) Pointers() map[string]*string {
	value := func(pointer *bool) *string {
		if pointer == nil {
			return nil
		}
		text := strconv.FormatBool(*pointer)
		return &text
	}
	return map[string]*string{
		"published":                                  value(v.Published),
		"supports_audio_descriptions":                value(v.SupportsAudioDescriptions),
		"supports_captions":                          value(v.SupportsCaptions),
		"supports_dark_interface":                    value(v.SupportsDarkInterface),
		"supports_differentiate_without_color_alone": value(v.SupportsDifferentiateWithoutColorAlone),
		"supports_larger_text":                       value(v.SupportsLargerText),
		"supports_reduced_motion":                    value(v.SupportsReducedMotion),
		"supports_sufficient_contrast":               value(v.SupportsSufficientContrast),
		"supports_voice_control":                     value(v.SupportsVoiceControl),
		"supports_voiceover":                         value(v.SupportsVoiceover),
	}
}

func (v *AccessibilityValues) SetManaged(field, value string) {
	parsed, _ := strconv.ParseBool(value)
	pointer := &parsed
	switch field {
	case "published":
		v.Published = pointer
	case "supports_audio_descriptions":
		v.SupportsAudioDescriptions = pointer
	case "supports_captions":
		v.SupportsCaptions = pointer
	case "supports_dark_interface":
		v.SupportsDarkInterface = pointer
	case "supports_differentiate_without_color_alone":
		v.SupportsDifferentiateWithoutColorAlone = pointer
	case "supports_larger_text":
		v.SupportsLargerText = pointer
	case "supports_reduced_motion":
		v.SupportsReducedMotion = pointer
	case "supports_sufficient_contrast":
		v.SupportsSufficientContrast = pointer
	case "supports_voice_control":
		v.SupportsVoiceControl = pointer
	case "supports_voiceover":
		v.SupportsVoiceover = pointer
	}
}

func SortedAccessibilityDeviceFamilies(values map[string]AccessibilityValues) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
