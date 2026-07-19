package screens

import (
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/internal/model"
)

func TestPersonaOptionsIncludeGentlemanNeutralArtifacts(t *testing.T) {
	options := PersonaOptions()
	found := false
	for _, option := range options {
		if option == model.PersonaGentlemanNeutralArtifacts {
			found = true
		}
	}
	if !found {
		t.Fatalf("PersonaOptions() = %v, missing %q", options, model.PersonaGentlemanNeutralArtifacts)
	}
}

func TestRenderPersonaDistinguishesConversationAndArtifactLanguage(t *testing.T) {
	tests := []struct {
		name    string
		persona model.PersonaID
		want    string
	}{
		{name: "Gentleman", persona: model.PersonaGentleman, want: "Voseo conversation; English technical artifacts"},
		{name: "Gentleman with English artifacts", persona: model.PersonaGentlemanNeutralArtifacts, want: "Voseo conversation; English technical artifacts"},
		{name: "Neutral", persona: model.PersonaNeutral, want: "No regional conversation tone; English technical artifacts"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := RenderPersona(tt.persona, 0)
			if !strings.Contains(out, tt.want) {
				t.Fatalf("RenderPersona() missing %q; output:\n%s", tt.want, out)
			}
		})
	}
}
