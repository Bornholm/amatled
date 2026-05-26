package format

import (
	"bytes"
	"strings"

	amatlmd "github.com/Bornholm/amatl/pkg/markdown/renderer/markdown"
	amatlmdnode "github.com/Bornholm/amatl/pkg/markdown/renderer/markdown/node"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

var gm = goldmark.New(goldmark.WithExtensions(extension.GFM))

// FormatMarkdown applique une passe de normalisation Markdown via le renderer amatl.
// Le contenu est parsé avec GFM puis rendu dans un style canonique.
// Retourne le contenu original si le formatage échoue, pour ne jamais corrompre le document.
func FormatMarkdown(content string) (string, error) {
	// Normalise les fins de ligne (\r\n → \n)
	src := []byte(strings.ReplaceAll(content, "\r\n", "\n"))

	reader := text.NewReader(src)
	root := gm.Parser().Parse(reader)

	r := amatlmd.NewRenderer()
	r.AddOptions(amatlmd.WithNodeRenderers(amatlmdnode.Renderers()))

	var buf bytes.Buffer
	if err := r.Render(&buf, src, root); err != nil {
		return content, err
	}

	result := buf.String()
	// Préserve le saut de ligne final si présent dans l'original
	if strings.HasSuffix(content, "\n") && !strings.HasSuffix(result, "\n") {
		result += "\n"
	}
	return result, nil
}
