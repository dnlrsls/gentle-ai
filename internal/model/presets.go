package model

// ComponentsForPreset returns the managed components implied by a preset/persona
// pair. PersonaCustom is a full opt-out from managed persona and managed visual
// polish, even when the user selects the full Gentleman preset.
func ComponentsForPreset(preset PresetID, persona PersonaID) []ComponentID {
	var components []ComponentID
	switch preset {
	case PresetMinimal:
		components = []ComponentID{ComponentEngram}
	case PresetEcosystemOnly:
		components = []ComponentID{ComponentEngram, ComponentSDD, ComponentSkills, ComponentContext7, ComponentGGA}
	case PresetCustom:
		return nil
	default: // full-gentleman
		components = []ComponentID{
			ComponentEngram,
			ComponentSDD,
			ComponentSkills,
			ComponentContext7,
			ComponentPermission,
			ComponentGGA,
		}
		if persona != PersonaCustom {
			components = append(components, ComponentClaudeTheme, ComponentOpenCodeGentleLogo)
		}
	}
	if persona != PersonaCustom {
		components = append(components, ComponentPersona)
	}
	return components
}
