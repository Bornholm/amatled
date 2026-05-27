package document

import (
	"strings"

	"github.com/bornholm/amatled/internal/workspace"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

type SectionEntry struct {
	Level int
	Title string
	Line  int
}

type FileIndex struct {
	Path     string
	Sections []SectionEntry
}

type WorkspaceIndex struct {
	Files []FileIndex
}

// BuildWorkspaceIndex scanne tous les .md du workspace et extrait les headings.
// Appelé à chaque run de l'agent pour garantir la fraîcheur de l'index.
func BuildWorkspaceIndex(ws *workspace.Workspace) (*WorkspaceIndex, error) {
	entries, err := ws.ListFiles()
	if err != nil {
		return nil, err
	}
	paths := flatFiles(entries)
	idx := &WorkspaceIndex{}
	for _, p := range paths {
		content, err := ws.ReadFile(p)
		if err != nil {
			continue
		}
		sections := extractSectionEntries(content)
		idx.Files = append(idx.Files, FileIndex{Path: p, Sections: sections})
	}
	return idx, nil
}

func flatFiles(entries []workspace.FileEntry) []string {
	var paths []string
	for _, e := range entries {
		if e.IsDir {
			paths = append(paths, flatFiles(e.Children)...)
		} else {
			paths = append(paths, e.Path)
		}
	}
	return paths
}

func extractSectionEntries(content string) []SectionEntry {
	src := []byte(content)
	reader := text.NewReader(src)
	doc := goldmark.New().Parser().Parse(reader)

	var entries []SectionEntry
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		h, ok := n.(*ast.Heading)
		if !ok {
			return ast.WalkContinue, nil
		}
		line := countNewlinesBefore(src, headingOffset(n))
		title := extractHeadingTitle(h, src)
		entries = append(entries, SectionEntry{Level: h.Level, Title: title, Line: line})
		return ast.WalkContinue, nil
	})
	return entries
}

func headingOffset(n ast.Node) int {
	if l := n.Lines(); l != nil && l.Len() > 0 {
		return l.At(0).Start
	}
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if l := c.Lines(); l != nil && l.Len() > 0 {
			return l.At(0).Start
		}
	}
	return 0
}

// FormatWorkspaceIndex retourne le TOC compact multi-fichiers sous forme de texte.
func FormatWorkspaceIndex(idx *WorkspaceIndex) string {
	if idx == nil || len(idx.Files) == 0 {
		return "(workspace vide)\n"
	}
	var sb strings.Builder
	for _, f := range idx.Files {
		sb.WriteString(f.Path)
		sb.WriteString(" :\n")
		for _, s := range f.Sections {
			indent := strings.Repeat("  ", s.Level)
			sb.WriteString(indent)
			sb.WriteString(strings.Repeat("#", s.Level))
			sb.WriteString(" ")
			sb.WriteString(s.Title)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}
