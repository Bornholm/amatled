package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/bornholm/amatled/internal/editor"
	"github.com/bornholm/amatled/internal/history"
	"github.com/bornholm/genai/llm"
)

func NewInsertBeforeSectionTool() llm.Tool {
	schema := llm.NewJSONSchema().
		RequiredProperty("content", "Le contenu Markdown à insérer avant la section active.", "string")

	return llm.NewFuncTool(
		"insert_before_section",
		"Insère du contenu Markdown avant la section actuellement sélectionnée.",
		schema,
		func(ctx context.Context, params map[string]any) (llm.ToolResult, error) {
			sc, ok := SessionContextFrom(ctx)
			if !ok {
				return nil, fmt.Errorf("session context manquant")
			}

			content, err := llm.ToolParam[string](params, "content")
			if err != nil {
				return nil, err
			}

			section := sc.Session.ActiveSection()
			if section == nil {
				return llm.NewToolResult(`{"error": "Aucune section active sélectionnée."}`), nil
			}

			if !strings.HasSuffix(content, "\n") {
				content += "\n"
			}

			insertPos := history.Position{Line: section.StartLine, Column: 0}
			change := editor.Change{
				From:   insertPos,
				To:     insertPos,
				Insert: content,
			}

			if err := sc.Session.ApplyChanges([]editor.Change{change}, history.SourceAgent, sc.AIMessage); err != nil {
				return nil, fmt.Errorf("apply insert_before_section: %w", err)
			}

			return llm.NewToolResult(fmt.Sprintf("Contenu inséré avant la section '%s'.", section.HeadingTitle)), nil
		},
	)
}
