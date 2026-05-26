package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/bornholm/genai/llm"
)

func NewListSectionsTool() llm.Tool {
	return llm.NewFuncTool(
		"list_sections",
		"Liste tous les titres (headings) du document actif avec leur niveau hiérarchique. Utile pour naviguer dans la structure du document.",
		llm.NewJSONSchema(),
		func(ctx context.Context, params map[string]any) (llm.ToolResult, error) {
			sc, ok := SessionContextFrom(ctx)
			if !ok {
				return nil, fmt.Errorf("session context manquant")
			}

			content := sc.Session.Content()
			headings := listHeadings(content)

			if len(headings) == 0 {
				return llm.NewToolResult("Le document ne contient aucun heading."), nil
			}

			var sb strings.Builder
			for _, h := range headings {
				indent := strings.Repeat("  ", h.HeadingLevel-1)
				sb.WriteString(fmt.Sprintf("%s- [L%d] %s %s\n",
					indent,
					h.StartLine,
					strings.Repeat("#", h.HeadingLevel),
					h.HeadingTitle,
				))
			}
			return llm.NewToolResult(sb.String()), nil
		},
	)
}
