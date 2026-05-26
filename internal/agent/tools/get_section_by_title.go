package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/bornholm/amatled/internal/document"
	"github.com/bornholm/genai/llm"
)

func NewGetSectionByTitleTool() llm.Tool {
	schema := llm.NewJSONSchema().
		RequiredProperty("title", "Le titre exact (ou partiel) du heading de la section à lire.", "string")

	return llm.NewFuncTool(
		"get_section_by_title",
		"Récupère le contenu Markdown d'une section identifiée par son titre. La recherche est insensible à la casse.",
		schema,
		func(ctx context.Context, params map[string]any) (llm.ToolResult, error) {
			sc, ok := SessionContextFrom(ctx)
			if !ok {
				return nil, fmt.Errorf("session context manquant")
			}

			title, err := llm.ToolParam[string](params, "title")
			if err != nil {
				return nil, err
			}

			content := sc.Session.Content()
			headings := listHeadings(content)
			lines := strings.Split(content, "\n")

			titleLower := strings.ToLower(title)
			var found *document.SectionRef
			for _, h := range headings {
				if strings.Contains(strings.ToLower(h.HeadingTitle), titleLower) {
					// Calculer l'endLine de cette section
					ref, err := document.ActiveSection(content, h.StartLine)
					if err == nil && ref != nil {
						found = ref
						break
					}
				}
			}

			if found == nil {
				return llm.NewToolResult(fmt.Sprintf(`{"error": "Aucune section dont le titre contient '%s' n'a été trouvée. Utilisez list_sections pour voir la liste complète."}`, title)), nil
			}

			_ = lines
			result := fmt.Sprintf("Section trouvée : %s %s (lignes %d-%d)\n\n%s",
				headingMarks(found.HeadingLevel),
				found.HeadingTitle,
				found.StartLine,
				found.EndLine,
				found.RawContent,
			)
			return llm.NewToolResult(result), nil
		},
	)
}
