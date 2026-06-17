package tools

import (
	"context"
	"fmt"

	"github.com/bornholm/genai/llm"
)

func NewReadSectionTool() llm.Tool {
	return llm.NewFuncTool(
		"read_section",
		"Lit le contenu Markdown brut de la section actuellement sélectionnée (heading + tout son contenu jusqu'au prochain heading de même niveau ou supérieur). Utilisez cet outil pour inspecter la section avant de la modifier.",
		llm.NewJSONSchema(),
		func(ctx context.Context, params map[string]any) (llm.ToolResult, error) {
			sc, ok := SessionContextFrom(ctx)
			if !ok {
				return nil, fmt.Errorf("session context manquant")
			}
			section := sc.Session.ActiveSection()
			if section == nil {
				return llm.NewToolResult("Aucune section active. Utilise list_sections pour lister les sections, puis get_section_by_title pour en lire une."), nil
			}
			result := fmt.Sprintf("Section active : %s %s (lignes %d-%d)\n\n%s",
				headingMarks(section.HeadingLevel),
				section.HeadingTitle,
				section.StartLine,
				section.EndLine,
				section.RawContent,
			)
			return llm.NewToolResult(result), nil
		},
	)
}

func headingMarks(level int) string {
	marks := ""
	for i := 0; i < level; i++ {
		marks += "#"
	}
	return marks
}
