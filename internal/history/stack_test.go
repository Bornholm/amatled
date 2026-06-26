package history_test

import (
	"testing"
	"time"

	"github.com/bornholm/amatled/internal/history"
)

// makeReplace crée un ReplaceRange qui remplace l'intégralité de oldContent par newContent.
func makeReplace(oldContent, newContent string) *history.ReplaceRange {
	return &history.ReplaceRange{
		Start:   history.Position{Line: 0, Column: 0},
		End:     endPos(oldContent),
		NewText: newContent,
	}
}

func endPos(content string) history.Position {
	lines := splitLines(content)
	last := len(lines) - 1
	if last < 0 {
		return history.Position{}
	}
	return history.Position{Line: last, Column: len([]rune(lines[last]))}
}

func splitLines(s string) []string {
	result := []string{}
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

func TestPushUndoRedo(t *testing.T) {
	stack := history.New("A", nil)

	// Push : A → B (attente > 500ms pour éviter le coalescing)
	time.Sleep(600 * time.Millisecond)
	if err := stack.Push(makeReplace("A", "B"), history.SourceHuman, ""); err != nil {
		t.Fatal(err)
	}
	if stack.Content() != "B" {
		t.Errorf("expected B, got %q", stack.Content())
	}

	// Push : B → C
	time.Sleep(600 * time.Millisecond)
	if err := stack.Push(makeReplace("B", "C"), history.SourceHuman, ""); err != nil {
		t.Fatal(err)
	}
	if stack.Content() != "C" {
		t.Errorf("expected C, got %q", stack.Content())
	}

	// Undo → B
	content, entry, err := stack.Undo()
	if err != nil {
		t.Fatal(err)
	}
	if content != "B" {
		t.Errorf("after undo expected B, got %q", content)
	}
	if entry.Source != history.SourceHuman {
		t.Errorf("expected SourceHuman, got %q", entry.Source)
	}

	// Undo → A
	content2, _, err2 := stack.Undo()
	if err2 != nil {
		t.Fatal(err2)
	}
	if content2 != "A" {
		t.Errorf("after second undo expected A, got %q", content2)
	}

	// Redo → B
	content3, _, err3 := stack.Redo()
	if err3 != nil {
		t.Fatal(err3)
	}
	if content3 != "B" {
		t.Errorf("after redo expected B, got %q", content3)
	}
}

func TestCoalescing(t *testing.T) {
	stack := history.New("", nil)

	// Deux pushes humains rapides → coalescés en une seule entrée
	if err := stack.Push(makeReplace("", "Hello"), history.SourceHuman, ""); err != nil {
		t.Fatal(err)
	}
	if err := stack.Push(makeReplace("Hello", "Hello!"), history.SourceHuman, ""); err != nil {
		t.Fatal(err)
	}
	if stack.Len() != 1 {
		t.Errorf("expected 1 entry (coalesced), got %d", stack.Len())
	}
	if stack.Content() != "Hello!" {
		t.Errorf("expected 'Hello!', got %q", stack.Content())
	}

	// Undo → "" (état avant la séquence)
	content, _, err := stack.Undo()
	if err != nil {
		t.Fatal(err)
	}
	if content != "" {
		t.Errorf("expected empty after undo, got %q", content)
	}
}

func TestAgentNotCoalesced(t *testing.T) {
	stack := history.New("X", nil)

	// Deux pushes agent → jamais coalescés
	if err := stack.Push(makeReplace("X", "A"), history.SourceAgent, "msg1"); err != nil {
		t.Fatal(err)
	}
	if err := stack.Push(makeReplace("A", "B"), history.SourceAgent, "msg2"); err != nil {
		t.Fatal(err)
	}
	if stack.Len() != 2 {
		t.Errorf("expected 2 entries (agent ops not coalesced), got %d", stack.Len())
	}
}

func TestRollbackTo(t *testing.T) {
	stack := history.New("v0", nil)

	time.Sleep(600 * time.Millisecond)
	if err := stack.Push(makeReplace("v0", "v1"), history.SourceHuman, ""); err != nil {
		t.Fatal(err)
	}
	time.Sleep(600 * time.Millisecond)
	if err := stack.Push(makeReplace("v1", "v2"), history.SourceAgent, "ai1"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(600 * time.Millisecond)
	if err := stack.Push(makeReplace("v2", "v3"), history.SourceHuman, ""); err != nil {
		t.Fatal(err)
	}

	if stack.Content() != "v3" {
		t.Fatalf("expected v3, got %q", stack.Content())
	}

	// Trouver l'ID de l'entrée v2 (index 1 dans la pile)
	// On annule v3 et v2 pour obtenir leur IDs
	_, entryV3, _ := stack.Undo() // revient à v2
	_, entryV2, _ := stack.Undo() // revient à v1
	stack.Redo()                  // revient à v2
	stack.Redo()                  // revient à v3
	_ = entryV3

	if stack.Content() != "v3" {
		t.Fatalf("expected v3 before rollback, got %q", stack.Content())
	}

	// RollbackTo v2 → annule v3 et v2 → revient à v1
	if err := stack.RollbackTo(entryV2.ID); err != nil {
		t.Fatal(err)
	}
	if stack.Content() != "v1" {
		t.Errorf("after rollback to v2, expected 'v1', got %q", stack.Content())
	}
}

func TestNothingToUndo(t *testing.T) {
	stack := history.New("X", nil)
	_, _, err := stack.Undo()
	if err != history.ErrNothingToUndo {
		t.Errorf("expected ErrNothingToUndo, got %v", err)
	}
}

func TestNothingToRedo(t *testing.T) {
	stack := history.New("X", nil)
	stack.Push(makeReplace("X", "Y"), history.SourceHuman, "")
	_, _, err := stack.Redo()
	if err != history.ErrNothingToRedo {
		t.Errorf("expected ErrNothingToRedo, got %v", err)
	}
}

func TestCommit(t *testing.T) {
	stack := history.New("v0", nil)

	stack.Push(makeReplace("v0", "v1"), history.SourceHuman, "")
	stack.Push(makeReplace("v1", "v2"), history.SourceHuman, "")

	if !stack.HasUncommitted() {
		t.Errorf("expected uncommitted changes")
	}

	stack.Commit()

	if stack.HasUncommitted() {
		t.Errorf("expected no uncommitted changes after commit")
	}

	if stack.CommittedContent() != "v2" {
		t.Errorf("expected committed content v2, got %q", stack.CommittedContent())
	}

	content, _, err := stack.Undo()
	if err != history.ErrNothingToUndo {
		t.Errorf("expected ErrNothingToUndo after commit, got %v", err)
	}
	_ = content
}

func TestDiscard(t *testing.T) {
	stack := history.New("v0", nil)

	stack.Push(makeReplace("v0", "v1"), history.SourceHuman, "")
	stack.Push(makeReplace("v1", "v2"), history.SourceHuman, "")

	content, err := stack.Discard()
	if err != nil {
		t.Fatal(err)
	}

	if content != "v0" {
		t.Errorf("expected content v0 after discard, got %q", content)
	}

	if stack.Content() != "v0" {
		t.Errorf("expected stack content v0, got %q", stack.Content())
	}

	if stack.Len() != 0 {
		t.Errorf("expected 0 entries after discard, got %d", stack.Len())
	}
}

func TestDiscardAfterUndo(t *testing.T) {
	stack := history.New("v0", nil)

	stack.Push(makeReplace("v0", "v1"), history.SourceHuman, "")
	stack.Push(makeReplace("v1", "v2"), history.SourceHuman, "")

	if _, _, err := stack.Undo(); err != nil {
		t.Fatal(err)
	}

	// After undo, current content is v1 and cursor is at 1.
	stack.Push(makeReplace("v1", "v3"), history.SourceHuman, "")

	content, err := stack.Discard()
	if err != nil {
		t.Fatal(err)
	}

	if content != "v0" {
		t.Errorf("expected content v0 after discard, got %q", content)
	}

	if stack.Len() != 0 {
		t.Errorf("expected 0 entries after discard, got %d", stack.Len())
	}
}

func TestRollbackCannotCrossCommitted(t *testing.T) {
	stack := history.New("v0", nil)

	stack.Push(makeReplace("v0", "v1"), history.SourceAgent, "msg1")

	// Retrieve the entry via undo then redo to restore the state.
	_, entryV1, err := stack.Undo()
	if err != nil {
		t.Fatal(err)
	}
	stack.Redo()

	// Commit the initial change.
	stack.Commit()

	// Add a new uncommitted change.
	stack.Push(makeReplace("v1", "v2"), history.SourceAgent, "msg2")

	// Rolling back to the now-committed entry should fail.
	err = stack.RollbackTo(entryV1.ID)
	if err != history.ErrCommittedState {
		t.Errorf("expected ErrCommittedState, got %v", err)
	}
}

