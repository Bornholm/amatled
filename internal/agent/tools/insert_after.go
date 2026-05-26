package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/bornholm/amatled/internal/editor"
	"github.com/bornholm/amatled/internal/history"
	"github.com/bornholm/genai/llm"
)

func NewInsertAfterSectionTool() llm.Tool {
	schema := llm.NewJSONSchema().
		RequiredProperty("content", "Le contenu Markdown à insérer après la section active.", "string")

	return llm.NewFuncTool(
		"insert_after_section",
		"Insère du contenu Markdown après la section actuellement sélectionnée (après sa dernière ligne).",
		schema,
		func(ctx context.Context, params map[string]any) (llm.ToolResult, error) {
			sc, ok := SessionContextFrom(ctx)
			if !ok {
				return nil, fmt.Errorf("session context manquant")
			}

			newContent, err := llm.ToolParam[string](params, "content")
			if err != nil {
				return nil, err
			}

			section := sc.Session.ActiveSection()
			if section == nil {
				return llm.NewToolResult(`{"error": "Aucune section active sélectionnée."}`), nil
			}

			if !strings.HasSuffix(newContent, "\n") {
				newContent += "\n"
			}

			docContent := sc.Session.Content()
			docLines := strings.Split(docContent, "\n")

			var insertPos history.Position
			if section.EndLine+1 < len(docLines) {
				insertPos = history.Position{Line: section.EndLine + 1, Column: 0}
			} else {
				lastLine := docLines[len(docLines)-1]
				insertPos = history.Position{Line: len(docLines) - 1, Column: len([]rune(lastLine))}
				newContent = "\n" + newContent
			}

			change := editor.Change{
				From:   insertPos,
				To:     insertPos,
				Insert: newContent,
			}

			if err := sc.Session.ApplyChanges([]editor.Change{change}, history.SourceAgent, sc.AIMessage); err != nil {
				return nil, fmt.Errorf("apply insert_after_section: %w", err)
			}

			return llm.NewToolResult(fmt.Sprintf("Contenu inséré après la section '%s'.", section.HeadingTitle)), nil
		},
	)
}
