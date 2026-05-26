package document

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// SectionRef décrit la section du document contenant le curseur.
type SectionRef struct {
	HeadingLevel int    `json:"headingLevel"`
	HeadingTitle string `json:"headingTitle"`
	StartLine    int    `json:"startLine"`
	EndLine      int    `json:"endLine"`
	RawContent   string `json:"rawContent"`
}

// headingEntry stocke un heading parsé avec sa ligne de début.
type headingEntry struct {
	level     int
	title     string
	startLine int
}

var mdParser = goldmark.New().Parser()

// ActiveSection retourne la section (heading + contenu) contenant cursorLine
// (0-indexed). Retourne nil si aucun heading ne précède le curseur.
func ActiveSection(content string, cursorLine int) (*SectionRef, error) {
	src := []byte(content)
	reader := text.NewReader(src)
	doc := mdParser.Parse(reader)

	var headings []headingEntry
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		h, ok := n.(*ast.Heading)
		if !ok {
			return ast.WalkContinue, nil
		}
		startLine := lineNumberOf(n, src)
		title := extractHeadingTitle(h, src)
		headings = append(headings, headingEntry{
			level:     h.Level,
			title:     title,
			startLine: startLine,
		})
		return ast.WalkContinue, nil
	})

	if len(headings) == 0 {
		return nil, nil
	}

	// Trouver le dernier heading dont startLine <= cursorLine.
	activeIdx := -1
	for i, h := range headings {
		if h.startLine <= cursorLine {
			activeIdx = i
		}
	}
	if activeIdx < 0 {
		return nil, nil
	}

	active := headings[activeIdx]

	// EndLine = ligne avant le prochain heading de niveau <= active.level, ou EOF.
	lines := strings.Split(content, "\n")
	totalLines := len(lines)
	endLine := totalLines - 1
	for _, h := range headings[activeIdx+1:] {
		if h.level <= active.level {
			endLine = h.startLine - 1
			break
		}
	}
	if endLine < active.startLine {
		endLine = active.startLine
	}

	raw := extractLines(lines, active.startLine, endLine)

	return &SectionRef{
		HeadingLevel: active.level,
		HeadingTitle: active.title,
		StartLine:    active.startLine,
		EndLine:      endLine,
		RawContent:   raw,
	}, nil
}

// lineNumberOf retourne le numéro de ligne (0-indexed) du premier segment d'un nœud.
func lineNumberOf(n ast.Node, src []byte) int {
	lines := n.Lines()
	if lines != nil && lines.Len() > 0 {
		seg := lines.At(0)
		return countNewlinesBefore(src, seg.Start)
	}
	// Fallback : chercher dans les enfants du nœud (pour les headings inline)
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		if child.Lines() != nil && child.Lines().Len() > 0 {
			seg := child.Lines().At(0)
			return countNewlinesBefore(src, seg.Start)
		}
	}
	return 0
}

func countNewlinesBefore(src []byte, offset int) int {
	count := 0
	for i := 0; i < offset && i < len(src); i++ {
		if src[i] == '\n' {
			count++
		}
	}
	return count
}

// extractHeadingTitle extrait le texte brut d'un heading.
func extractHeadingTitle(h *ast.Heading, src []byte) string {
	var sb strings.Builder
	for child := h.FirstChild(); child != nil; child = child.NextSibling() {
		if t, ok := child.(*ast.Text); ok {
			seg := t.Segment
			sb.Write(src[seg.Start:seg.Stop])
		}
	}
	return sb.String()
}

// extractLines extrait les lignes [from, to] incluses d'un document.
func extractLines(lines []string, from, to int) string {
	if from > to || from >= len(lines) {
		return ""
	}
	if to >= len(lines) {
		to = len(lines) - 1
	}
	return strings.Join(lines[from:to+1], "\n")
}
