package document_test

import (
	"testing"

	"github.com/bornholm/amatled/internal/document"
)

func TestActiveSectionBasic(t *testing.T) {
	content := `# Titre principal

Intro du document.

## Section A

Contenu de la section A.

Paragraphe 2 de A.

## Section B

Contenu de la section B.

### Sous-section B1

Contenu de B1.
`
	tests := []struct {
		name          string
		cursorLine    int
		wantLevel     int
		wantTitle     string
		wantNil       bool
	}{
		{"avant tout heading", 0, 0, "", false}, // ligne 0 = # Titre principal
		{"dans l'intro", 2, 1, "Titre principal", false},
		{"dans section A", 6, 2, "Section A", false},
		{"dans section B", 12, 2, "Section B", false},
		{"dans sous-section B1", 16, 3, "Sous-section B1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := document.ActiveSection(content, tt.cursorLine)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNil {
				if ref != nil {
					t.Errorf("expected nil, got %+v", ref)
				}
				return
			}
			if ref == nil {
				t.Fatalf("expected non-nil SectionRef for cursor line %d", tt.cursorLine)
			}
			if tt.wantLevel > 0 && ref.HeadingLevel != tt.wantLevel {
				t.Errorf("level: want %d, got %d", tt.wantLevel, ref.HeadingLevel)
			}
			if tt.wantTitle != "" && ref.HeadingTitle != tt.wantTitle {
				t.Errorf("title: want %q, got %q", tt.wantTitle, ref.HeadingTitle)
			}
		})
	}
}

func TestActiveSectionEmpty(t *testing.T) {
	ref, err := document.ActiveSection("", 0)
	if err != nil {
		t.Fatal(err)
	}
	if ref != nil {
		t.Errorf("expected nil for empty document, got %+v", ref)
	}
}

func TestActiveSectionNoHeadings(t *testing.T) {
	content := "Juste du texte\nsans aucun heading\n"
	ref, err := document.ActiveSection(content, 1)
	if err != nil {
		t.Fatal(err)
	}
	if ref != nil {
		t.Errorf("expected nil for document without headings, got %+v", ref)
	}
}

func TestActiveSectionEndLine(t *testing.T) {
	content := "# A\n\nContenu A\n\n# B\n\nContenu B\n"
	// Curseur dans A (ligne 2) → EndLine doit être avant # B (ligne 4)
	ref, err := document.ActiveSection(content, 2)
	if err != nil {
		t.Fatal(err)
	}
	if ref == nil {
		t.Fatal("expected non-nil")
	}
	if ref.HeadingTitle != "A" {
		t.Errorf("expected title A, got %q", ref.HeadingTitle)
	}
	if ref.EndLine >= 4 {
		t.Errorf("EndLine %d should be < 4 (start of next H1)", ref.EndLine)
	}
}
