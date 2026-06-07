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
//
// Un éventuel bloc frontmatter (YAML/TOML délimité par une ligne de `---` ou `+++`
// répétée en tête de document) n'est pas du Markdown : il est extrait avant
// normalisation et recollé tel quel, pour ne pas être corrompu par le pipeline
// de rendu Goldmark/amatl qui ne sait pas l'interpréter.
func FormatMarkdown(content string) (string, error) {
	// Normalise les fins de ligne (\r\n → \n)
	normalized := strings.ReplaceAll(content, "\r\n", "\n")

	frontMatter, body := splitFrontMatter(normalized)

	src := []byte(body)
	reader := text.NewReader(src)
	root := gm.Parser().Parse(reader)

	r := amatlmd.NewRenderer()
	r.AddOptions(amatlmd.WithNodeRenderers(amatlmdnode.Renderers()))

	var buf bytes.Buffer
	if err := r.Render(&buf, src, root); err != nil {
		return content, err
	}

	result := buf.String()
	if frontMatter != "" {
		if trimmed := strings.TrimLeft(result, "\n"); trimmed == "" {
			result = frontMatter
		} else {
			result = frontMatter + "\n" + trimmed
		}
	}

	// Préserve le saut de ligne final si présent dans l'original
	if strings.HasSuffix(content, "\n") && !strings.HasSuffix(result, "\n") {
		result += "\n"
	}
	return result, nil
}

// splitFrontMatter sépare un éventuel bloc frontmatter (délimité par une ligne
// de tirets ou de signes plus, répétée au moins 3 fois, en tout début de
// document, et refermée par une ligne identique) du reste du contenu.
//
// Si aucun bloc frontmatter valide n'est détecté, frontMatter est vide et body
// vaut content.
func splitFrontMatter(content string) (frontMatter string, body string) {
	lines := strings.SplitAfter(content, "\n")
	if len(lines) == 0 {
		return "", content
	}

	openDelim, openCount := frontMatterDelim(lines[0])
	if openDelim == 0 {
		return "", content
	}

	for i := 1; i < len(lines); i++ {
		delim, count := frontMatterDelim(lines[i])
		if delim == openDelim && count == openCount {
			return strings.Join(lines[:i+1], ""), strings.Join(lines[i+1:], "")
		}
	}

	return "", content
}

// frontMatterDelim détecte une ligne de délimitation de frontmatter, c'est à
// dire une ligne ne contenant que le même caractère ('-' ou '+') répété au
// moins 3 fois. Retourne le caractère et son nombre de répétitions, ou
// (0, 0) si la ligne n'est pas un délimiteur valide.
func frontMatterDelim(line string) (delim byte, count int) {
	trimmed := strings.TrimRight(line, "\r\n")
	if len(trimmed) < 3 {
		return 0, 0
	}

	delim = trimmed[0]
	if delim != '-' && delim != '+' {
		return 0, 0
	}

	for i := 0; i < len(trimmed); i++ {
		if trimmed[i] != delim {
			return 0, 0
		}
	}

	return delim, len(trimmed)
}
