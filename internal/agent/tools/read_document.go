package tools

import (
	"context"
	"fmt"

	"github.com/bornholm/genai/llm"
)

const maxDocumentChars = 50_000

func NewReadDocumentTool() llm.Tool {
	return llm.NewFuncTool(
		"read_document",
		"Lit le contenu Markdown complet du fichier actif. Si le document est très long, il peut être tronqué.",
		llm.NewJSONSchema(),
		func(ctx context.Context, params map[string]any) (llm.ToolResult, error) {
			sc, ok := SessionContextFrom(ctx)
			if !ok {
				return nil, fmt.Errorf("session context manquant")
			}
			content := sc.Session.Content()
			if len(content) > maxDocumentChars {
				content = content[:maxDocumentChars] + "\n\n[... document tronqué ...]"
			}
			return llm.NewToolResult(content), nil
		},
	)
}
