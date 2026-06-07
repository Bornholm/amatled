package agent

import (
	"strings"
	"testing"

	"github.com/bornholm/amatled/internal/editor"
)

func TestBuildSystemPromptTruncatesLargeSection(t *testing.T) {
	heading := "# Grosse section\n\n"
	body := strings.Repeat("Ligne de contenu très longue qui occupe de la place.\n", 2000)
	content := heading + body

	mgr := editor.NewSessionManager(nil)
	sess := mgr.Open("doc.md", content)
	if _, err := sess.SetCursorLine(1); err != nil {
		t.Fatalf("set cursor line: %v", err)
	}
	if sess.ActiveSection() == nil {
		t.Fatalf("expected an active section")
	}
	if len(sess.ActiveSection().RawContent) <= maxInlinedSectionChars {
		t.Fatalf("test setup invalid: section content (%d chars) must exceed maxInlinedSectionChars (%d)",
			len(sess.ActiveSection().RawContent), maxInlinedSectionChars)
	}

	prompt := buildSystemPrompt(sess, nil, "")

	if !strings.Contains(prompt, "contenu tronqué") {
		t.Errorf("expected system prompt to mention truncation, got prompt of length %d", len(prompt))
	}
	if len(prompt) > maxInlinedSectionChars+10_000 {
		t.Errorf("expected system prompt to stay bounded, got length %d", len(prompt))
	}
}

func TestBuildSystemPromptKeepsSmallSectionIntact(t *testing.T) {
	content := "# Petite section\n\nContenu court.\n"

	mgr := editor.NewSessionManager(nil)
	sess := mgr.Open("doc.md", content)
	if _, err := sess.SetCursorLine(1); err != nil {
		t.Fatalf("set cursor line: %v", err)
	}

	prompt := buildSystemPrompt(sess, nil, "")

	if strings.Contains(prompt, "contenu tronqué") {
		t.Errorf("did not expect truncation mention for a small section")
	}
	if !strings.Contains(prompt, "Contenu court.") {
		t.Errorf("expected system prompt to contain the full section content")
	}
}
