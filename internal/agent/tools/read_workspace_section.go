package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/bornholm/amatled/internal/document"
	"github.com/bornholm/genai/llm"
)

func NewReadWorkspaceSectionTool() llm.Tool {
	schema := llm.NewJSONSchema().
		RequiredProperty("file_path", "Chemin relatif du fichier Markdown dans le workspace (ex: docs/intro.md).", "string").
		RequiredProperty("section_title", "Titre (partiel, insensible à la casse) de la section à lire.", "string")

	return llm.NewFuncTool(
		"read_workspace_section",
		"Lit le contenu d'une section d'un fichier Markdown du workspace, identifiée par son titre. Utilise list_workspace_sections pour connaître les titres disponibles.",
		schema,
		func(ctx context.Context, params map[string]any) (llm.ToolResult, error) {
			sc, ok := SessionContextFrom(ctx)
			if !ok {
				return nil, fmt.Errorf("session context manquant")
			}
			if sc.Workspace == nil {
				return llm.NewToolResult("Aucun workspace ouvert."), nil
			}

			filePath, err := llm.ToolParam[string](params, "file_path")
			if err != nil {
				return nil, err
			}
			sectionTitle, err := llm.ToolParam[string](params, "section_title")
			if err != nil {
				return nil, err
			}

			content, err := sc.Workspace.ReadFile(filePath)
			if err != nil {
				return llm.NewToolResult(fmt.Sprintf(`{"error": "Fichier introuvable : %s. Utilise list_workspace_sections pour voir les fichiers disponibles."}`, filePath)), nil
			}

			headings := listHeadings(content)
			titleLower := strings.ToLower(sectionTitle)

			var found *document.SectionRef
			for _, h := range headings {
				if strings.Contains(strings.ToLower(h.HeadingTitle), titleLower) {
					ref, err := document.ActiveSection(content, h.StartLine)
					if err == nil && ref != nil {
						found = ref
						break
					}
				}
			}

			if found == nil {
				return llm.NewToolResult(fmt.Sprintf(
					`{"error": "Aucune section dont le titre contient '%s' dans '%s'. Utilise list_workspace_sections pour voir les sections disponibles."}`,
					sectionTitle, filePath,
				)), nil
			}

			result := fmt.Sprintf("Fichier : %s\nSection : %s %s (lignes %d-%d)\n\n%s",
				filePath,
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
