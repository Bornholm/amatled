package render

import (
	"context"
	"path/filepath"

	amatlrender "github.com/Bornholm/amatl/pkg/command/cli/render"
	"github.com/Bornholm/amatl/pkg/pipeline"
	"github.com/Bornholm/amatl/pkg/resolver"
	_ "github.com/Bornholm/amatl/pkg/resolver/all"
)

// NormalizeMarkdown passe le contenu Markdown par le pipeline de normalisation
// amatl (parse AST → re-rendu canonique) sans déplier les directives :include.
func NormalizeMarkdown(ctx context.Context, content []byte, filePath, workspaceRoot string) ([]byte, error) {
	sourceDir := filepath.Dir(filePath)
	if sourceDir == "." || sourceDir == "" {
		sourceDir = workspaceRoot
	}

	sourcePath := resolver.Path(sourceDir)
	t := pipeline.Pipeline(
		amatlrender.MarkdownMiddleware(
			amatlrender.WithSourcePath(sourcePath),
		),
	)

	payload := pipeline.NewPayload(content)
	if err := t.Transform(ctx, payload); err != nil {
		return nil, err
	}
	return payload.GetData(), nil
}

// RenderHTML transforme du Markdown (avec directives amatl) en HTML complet.
// filePath est le chemin absolu du fichier source (pour résoudre les :include).
// workspaceRoot sert de fallback si filePath est absent.
func RenderHTML(ctx context.Context, content []byte, filePath, workspaceRoot string) (string, error) {
	sourceDir := filepath.Dir(filePath)
	if sourceDir == "." || sourceDir == "" {
		sourceDir = workspaceRoot
	}

	sourcePath := resolver.Path(sourceDir)
	t := pipeline.Pipeline(
		amatlrender.HTMLMiddleware(
			amatlrender.WithMarkdownTransformerOptions(
				amatlrender.WithSourcePath(sourcePath),
			),
			amatlrender.WithLayoutURL("amatl://document.html"),
		),
	)

	payload := pipeline.NewPayload(content)
	if err := t.Transform(ctx, payload); err != nil {
		return "", err
	}
	return string(payload.GetData()), nil
}
