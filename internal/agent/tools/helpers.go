package tools

import (
	"fmt"
	"strings"

	"github.com/bornholm/amatled/internal/document"
	"github.com/bornholm/amatled/internal/history"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// sectionBounds retourne les positions Start/End pour couvrir les lignes [startLine, endLine]
// du contenu donné (inclus aux deux bouts, avec le \n de fin d'endLine).
func sectionBounds(content string, startLine, endLine int) (startPos, endPos history.Position, err error) {
	lines := strings.Split(content, "\n")
	if startLine < 0 || startLine >= len(lines) {
		return startPos, endPos, fmt.Errorf("startLine %d hors plage (document %d lignes)", startLine, len(lines))
	}
	startPos = history.Position{Line: startLine, Column: 0}
	// End = début de la ligne suivante (inclut le \n d'endLine)
	if endLine+1 < len(lines) {
		endPos = history.Position{Line: endLine + 1, Column: 0}
	} else {
		// Dernière ligne : fin du document
		lastLine := lines[len(lines)-1]
		endPos = history.Position{Line: len(lines) - 1, Column: len([]rune(lastLine))}
	}
	return
}

// listHeadings retourne tous les headings d'un document Markdown.
func listHeadings(content string) []document.SectionRef {
	src := []byte(content)
	reader := text.NewReader(src)
	doc := goldmark.New().Parser().Parse(reader)
	lines := strings.Split(content, "\n")

	var refs []document.SectionRef
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		h, ok := n.(*ast.Heading)
		if !ok {
			return ast.WalkContinue, nil
		}
		startLine := lineOf(n, src)
		title := headingTitle(h, src)
		refs = append(refs, document.SectionRef{
			HeadingLevel: h.Level,
			HeadingTitle: title,
			StartLine:    startLine,
			EndLine:      startLine,
			RawContent:   lines[startLine],
		})
		return ast.WalkContinue, nil
	})
	return refs
}

func lineOf(n ast.Node, src []byte) int {
	if l := n.Lines(); l != nil && l.Len() > 0 {
		seg := l.At(0)
		count := 0
		for i := 0; i < seg.Start && i < len(src); i++ {
			if src[i] == '\n' {
				count++
			}
		}
		return count
	}
	return 0
}

func headingTitle(h *ast.Heading, src []byte) string {
	var sb strings.Builder
	for c := h.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			seg := t.Segment
			sb.Write(src[seg.Start:seg.Stop])
		}
	}
	return sb.String()
}
