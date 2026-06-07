package format_test

import (
	"strings"
	"testing"

	"github.com/bornholm/amatled/internal/format"
)

func TestFormatMarkdownPreservesYAMLFrontMatter(t *testing.T) {
	content := `---
title: mon Titre
subtitle: mon Sous Titre
dateupdate: 03/06/2026
version: V1.1
author: xxx
---

# Titre 01

Un paragraphe.
`

	result, err := format.FormatMarkdown(content)
	if err != nil {
		t.Fatalf("FormatMarkdown a retourné une erreur: %v", err)
	}

	frontMatter := `---
title: mon Titre
subtitle: mon Sous Titre
dateupdate: 03/06/2026
version: V1.1
author: xxx
---
`

	if !strings.HasPrefix(result, frontMatter) {
		t.Fatalf("le frontmatter n'a pas été préservé tel quel, résultat:\n%s", result)
	}

	if !strings.Contains(result, "# Titre 01") {
		t.Fatalf("le corps du document a disparu, résultat:\n%s", result)
	}
}

func TestFormatMarkdownPreservesTOMLFrontMatter(t *testing.T) {
	content := `+++
title = "mon Titre"
+++

# Titre 01
`

	result, err := format.FormatMarkdown(content)
	if err != nil {
		t.Fatalf("FormatMarkdown a retourné une erreur: %v", err)
	}

	frontMatter := "+++\ntitle = \"mon Titre\"\n+++\n"
	if !strings.HasPrefix(result, frontMatter) {
		t.Fatalf("le frontmatter TOML n'a pas été préservé tel quel, résultat:\n%s", result)
	}
}

func TestFormatMarkdownWithoutFrontMatterIsNormalizedAsBefore(t *testing.T) {
	content := "#Titre sans espace\n"

	result, err := format.FormatMarkdown(content)
	if err != nil {
		t.Fatalf("FormatMarkdown a retourné une erreur: %v", err)
	}

	if strings.HasPrefix(result, "---") {
		t.Fatalf("un frontmatter a été détecté à tort, résultat:\n%s", result)
	}

	if !strings.Contains(result, "Titre sans espace") {
		t.Fatalf("le contenu a disparu après normalisation, résultat:\n%s", result)
	}
}

func TestFormatMarkdownWithUnterminatedFrontMatterFallsBack(t *testing.T) {
	content := `---
title: mon Titre

# Titre 01
`

	result, err := format.FormatMarkdown(content)
	if err != nil {
		t.Fatalf("FormatMarkdown a retourné une erreur: %v", err)
	}

	if result == "" {
		t.Fatalf("le résultat ne devrait pas être vide")
	}
}
