package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/bornholm/amatled/internal/editor"
	"github.com/bornholm/amatled/internal/history"
	"github.com/bornholm/genai/llm"
)

func NewReplaceSectionTool() llm.Tool {
	schema := llm.NewJSONSchema().
		RequiredProperty("new_content", "Le nouveau contenu Markdown complet de la section (heading inclus). Doit conserver le niveau de heading d'origine.", "string")

	return llm.NewFuncTool(
		"replace_section",
		"Remplace l'intégralité du contenu de la section actuellement sélectionnée par le contenu fourni. L'opération est enregistrée dans l'historique d'annulation.",
		schema,
		func(ctx context.Context, params map[string]any) (llm.ToolResult, error) {
			sc, ok := SessionContextFrom(ctx)
			if !ok {
				return nil, fmt.Errorf("session context manquant")
			}

			newContent, err := llm.ToolParam[string](params, "new_content")
			if err != nil {
				return nil, err
			}

			section := sc.Session.ActiveSection()
			if section == nil {
				return llm.NewToolResult(`{"error": "Aucune section active sélectionnée. Positionnez le curseur dans une section d'abord."}`), nil
			}

			content := sc.Session.Content()
			startPos, endPos, err := sectionBounds(content, section.StartLine, section.EndLine)
			if err != nil {
				return llm.NewToolResult(fmt.Sprintf(`{"error": "%s"}`, err.Error())), nil
			}

			if !strings.HasSuffix(newContent, "\n") {
				newContent += "\n"
			}

			change := editor.Change{
				From:   startPos,
				To:     endPos,
				Insert: newContent,
			}

			if err := sc.Session.ApplyChanges([]editor.Change{change}, history.SourceAgent, sc.AIMessage); err != nil {
				return nil, fmt.Errorf("apply replace_section: %w", err)
			}

			return llm.NewToolResult(fmt.Sprintf("Section '%s' remplacée avec succès (%d caractères).", section.HeadingTitle, len(newContent))), nil
		},
	)
}
