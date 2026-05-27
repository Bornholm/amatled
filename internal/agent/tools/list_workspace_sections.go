package tools

import (
	"context"
	"fmt"

	"github.com/bornholm/amatled/internal/document"
	"github.com/bornholm/genai/llm"
)

func NewListWorkspaceSectionsTool() llm.Tool {
	return llm.NewFuncTool(
		"list_workspace_sections",
		"Liste tous les fichiers Markdown du workspace avec leur table des matières (headings hiérarchiques). Utilise cet outil pour découvrir le contenu disponible avant de lire une section précise.",
		llm.NewJSONSchema(),
		func(ctx context.Context, _ map[string]any) (llm.ToolResult, error) {
			sc, ok := SessionContextFrom(ctx)
			if !ok {
				return nil, fmt.Errorf("session context manquant")
			}
			if sc.Workspace == nil {
				return llm.NewToolResult("Aucun workspace ouvert."), nil
			}
			idx, err := document.BuildWorkspaceIndex(sc.Workspace)
			if err != nil {
				return nil, fmt.Errorf("build workspace index: %w", err)
			}
			return llm.NewToolResult(document.FormatWorkspaceIndex(idx)), nil
		},
	)
}
