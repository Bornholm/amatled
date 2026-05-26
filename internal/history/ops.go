package history

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrInvalidRange    = errors.New("invalid range")
	ErrNothingToUndo   = errors.New("nothing to undo")
	ErrNothingToRedo   = errors.New("nothing to redo")
	ErrEntryNotFound   = errors.New("entry not found")
)

// Source identifie l'auteur d'une modification.
type Source string

const (
	SourceHuman Source = "human"
	SourceAgent Source = "agent"
)

// Position désigne une position dans un document texte (0-indexed).
type Position struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

func (p Position) String() string {
	return fmt.Sprintf("%d:%d", p.Line, p.Column)
}

// Op est l'interface d'une opération réversible sur un document texte.
type Op interface {
	Type() string
	Apply(content string) (string, error)
	Inverse(content string) (Op, error)
	Describe() string
}

// ReplaceRange remplace le texte entre Start et End par NewText.
type ReplaceRange struct {
	Start   Position `json:"start"`
	End     Position `json:"end"`
	NewText string   `json:"newText"`
}

func (o *ReplaceRange) Type() string { return "replace_range" }

func (o *ReplaceRange) Apply(content string) (string, error) {
	startOff, err := offsetOf(content, o.Start)
	if err != nil {
		return "", fmt.Errorf("start offset: %w", err)
	}
	endOff, err := offsetOf(content, o.End)
	if err != nil {
		return "", fmt.Errorf("end offset: %w", err)
	}
	if startOff > endOff || endOff > len(content) {
		return "", ErrInvalidRange
	}
	return content[:startOff] + o.NewText + content[endOff:], nil
}

func (o *ReplaceRange) Inverse(content string) (Op, error) {
	startOff, err := offsetOf(content, o.Start)
	if err != nil {
		return nil, err
	}
	endOff, err := offsetOf(content, o.End)
	if err != nil {
		return nil, err
	}
	if startOff > endOff || endOff > len(content) {
		return nil, ErrInvalidRange
	}
	oldText := content[startOff:endOff]
	newContent := content[:startOff] + o.NewText + content[endOff:]
	endAfter := positionAfter(newContent[:startOff+len(o.NewText)])
	return &ReplaceRange{
		Start:   o.Start,
		End:     endAfter,
		NewText: oldText,
	}, nil
}

func (o *ReplaceRange) Describe() string {
	return fmt.Sprintf("replace %v-%v with %d chars", o.Start, o.End, len(o.NewText))
}

// tryMerge tente de fusionner deux ops ReplaceRange consécutives.
// Dans le cas Phase 2 (whole-doc replace : Start=0:0), retourne une op
// qui, appliquée au document original (avant ra), produit le résultat de rb.
// Retourne l'op fusionnée et true si la fusion est possible.
func tryMerge(a, b Op) (Op, bool) {
	ra, okA := a.(*ReplaceRange)
	rb, okB := b.(*ReplaceRange)
	if !okA || !okB {
		return nil, false
	}
	// Fusion simple : deux whole-doc replaces (Start=0:0).
	// On construit un op qui part du début du doc ORIGINAL (End=ra.End)
	// et le remplace par le texte final (rb.NewText).
	zeroPos := Position{Line: 0, Column: 0}
	if ra.Start == zeroPos && rb.Start == zeroPos {
		return &ReplaceRange{Start: ra.Start, End: ra.End, NewText: rb.NewText}, true
	}
	return nil, false
}

// offsetOf convertit une position ligne:colonne en offset byte dans content.
func offsetOf(content string, pos Position) (int, error) {
	if pos.Line == 0 && pos.Column == 0 {
		return 0, nil
	}
	line := 0
	offset := 0
	for offset < len(content) {
		if line == pos.Line {
			// Avancer de pos.Column runes
			col := 0
			for col < pos.Column && offset < len(content) {
				r := rune(content[offset])
				if r >= 0x80 {
					// Décoder le caractère multi-octet
					runeLen := 1
					for runeLen < 4 && offset+runeLen < len(content) && (content[offset+runeLen]&0xC0) == 0x80 {
						runeLen++
					}
					offset += runeLen
				} else {
					offset++
				}
				col++
			}
			if col < pos.Column {
				return 0, fmt.Errorf("column %d out of range on line %d", pos.Column, pos.Line)
			}
			return offset, nil
		}
		if content[offset] == '\n' {
			line++
		}
		offset++
	}
	// Position = fin du document
	if pos.Line == line && pos.Column == 0 {
		return len(content), nil
	}
	return 0, fmt.Errorf("line %d out of range (document has %d lines)", pos.Line, line)
}

// positionAfter retourne la position (ligne:colonne) à la fin du texte fourni.
func positionAfter(text string) Position {
	line := 0
	col := 0
	for _, r := range text {
		if r == '\n' {
			line++
			col = 0
		} else {
			col++
		}
	}
	return Position{Line: line, Column: col}
}

// newID génère un identifiant unique simple (hex aléatoire).
func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// EndPosition retourne la position correspondant à la fin du contenu.
func EndPosition(content string) Position {
	lines := strings.Split(content, "\n")
	lastLine := len(lines) - 1
	return Position{Line: lastLine, Column: len([]rune(lines[lastLine]))}
}
