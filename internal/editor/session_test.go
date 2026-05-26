package editor_test

import (
	"testing"
	"time"

	"github.com/bornholm/amatled/internal/editor"
	"github.com/bornholm/amatled/internal/history"
)

func pos(line, col int) history.Position {
	return history.Position{Line: line, Column: col}
}

// replaceAll crée un Change qui remplace l'intégralité de oldContent par newContent.
func replaceAll(oldContent, newContent string) editor.Change {
	lines := splitLines(oldContent)
	last := len(lines) - 1
	return editor.Change{
		From:   pos(0, 0),
		To:     pos(last, len([]rune(lines[last]))),
		Insert: newContent,
	}
}

func splitLines(s string) []string {
	var result []string
	cur := ""
	for _, r := range s {
		if r == '\n' {
			result = append(result, cur)
			cur = ""
		} else {
			cur += string(r)
		}
	}
	result = append(result, cur)
	return result
}

func TestSessionOpenApplyUndo(t *testing.T) {
	mgr := editor.NewSessionManager(nil)
	sess := mgr.Open("test.md", "initial")

	if sess.Content() != "initial" {
		t.Fatalf("expected 'initial', got %q", sess.Content())
	}

	// Apply change (attente > 500ms pour éviter le coalescing avec le state initial)
	time.Sleep(600 * time.Millisecond)
	if err := sess.ApplyChanges([]editor.Change{replaceAll("initial", "modified")}, history.SourceHuman, ""); err != nil {
		t.Fatal(err)
	}
	if sess.Content() != "modified" {
		t.Fatalf("expected 'modified', got %q", sess.Content())
	}

	// Undo
	content, entry, err := sess.Undo()
	if err != nil {
		t.Fatal(err)
	}
	if content != "initial" {
		t.Errorf("after undo expected 'initial', got %q", content)
	}
	if entry == nil {
		t.Error("expected non-nil entry")
	}
}

func TestSessionManagerGetClose(t *testing.T) {
	mgr := editor.NewSessionManager(nil)
	mgr.Open("a.md", "content A")
	mgr.Open("b.md", "content B")

	if _, ok := mgr.Get("a.md"); !ok {
		t.Error("expected session for a.md")
	}
	mgr.Close("a.md")
	if _, ok := mgr.Get("a.md"); ok {
		t.Error("expected session for a.md to be closed")
	}
	if _, ok := mgr.Get("b.md"); !ok {
		t.Error("expected session for b.md to still exist")
	}
}

func TestSessionSetCursorLine(t *testing.T) {
	content := "# Hello\n\nSome text here.\n"
	mgr := editor.NewSessionManager(nil)
	sess := mgr.Open("test.md", content)

	ref, err := sess.SetCursorLine(2)
	if err != nil {
		t.Fatal(err)
	}
	if ref == nil {
		t.Fatal("expected non-nil SectionRef")
	}
	if ref.HeadingTitle != "Hello" {
		t.Errorf("expected title 'Hello', got %q", ref.HeadingTitle)
	}
}

func TestSessionLockSection(t *testing.T) {
	content := "# A\n\n## B\n\nTexte.\n"
	mgr := editor.NewSessionManager(nil)
	sess := mgr.Open("test.md", content)

	// Détecter section sur ligne 4 → Section B
	ref1, _ := sess.SetCursorLine(4)
	if ref1 == nil || ref1.HeadingTitle != "B" {
		t.Fatalf("expected section B, got %v", ref1)
	}

	// Verrouiller
	sess.LockSection(true)

	// Déplacer curseur sur ligne 0 → section verrouillée, retourne toujours B
	ref2, _ := sess.SetCursorLine(0)
	if ref2 == nil || ref2.HeadingTitle != "B" {
		t.Errorf("locked section should stay on B, got %v", ref2)
	}

	// Déverrouiller → recalcul
	sess.LockSection(false)
	ref3, _ := sess.SetCursorLine(0)
	if ref3 == nil || ref3.HeadingTitle != "A" {
		t.Errorf("unlocked section should update to A, got %v", ref3)
	}
}
