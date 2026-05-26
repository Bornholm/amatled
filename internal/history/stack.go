package history

import (
	"sync"
	"time"
)

// Entry est une entrée dans la pile d'historique.
type Entry struct {
	ID          string    `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	Source      Source    `json:"source"`
	AIMessageID string    `json:"aiMessageId,omitempty"`
	Op          Op        `json:"-"`
	InverseOp   Op        `json:"-"`
	Coalescable bool      `json:"-"`
	Description string    `json:"description"`
}

// Direction indique le sens d'application dans l'historique.
type Direction int

const (
	DirForward Direction = iota
	DirUndo
	DirRedo
)

// OnChangeFn est appelée après chaque mutation du contenu.
type OnChangeFn func(content string, entry Entry, direction Direction)

// Stack est la pile d'historique d'un document.
// Toute mutation doit transiter par Push.
// Thread-safe.
type Stack struct {
	mu       sync.Mutex
	entries  []Entry
	cursor   int
	content  string
	capacity int
	onChange OnChangeFn
}

// New crée un Stack initialisé avec un contenu et un callback de notification.
func New(initialContent string, onChange OnChangeFn) *Stack {
	return &Stack{
		content:  initialContent,
		capacity: 500,
		onChange: onChange,
	}
}

// Content retourne le contenu courant (source of truth).
func (s *Stack) Content() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.content
}

// Push applique op au contenu courant et l'empile.
// Si src == SourceHuman et que la dernière entrée est coalescable et < 500ms,
// les deux ops sont fusionnées.
func (s *Stack) Push(op Op, src Source, aiMsgID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	inv, err := op.Inverse(s.content)
	if err != nil {
		return err
	}
	newContent, err := op.Apply(s.content)
	if err != nil {
		return err
	}

	now := time.Now()
	entry := Entry{
		ID:          newID(),
		Timestamp:   now,
		Source:      src,
		AIMessageID: aiMsgID,
		Op:          op,
		InverseOp:   inv,
		Coalescable: src == SourceHuman,
		Description: op.Describe(),
	}

	// Tronque la branche redo dès qu'une nouvelle op est poussée.
	s.entries = s.entries[:s.cursor]

	// Coalescing : fusionne avec la dernière entrée humaine si < 500ms.
	if s.cursor > 0 && entry.Coalescable {
		last := &s.entries[s.cursor-1]
		if last.Coalescable && now.Sub(last.Timestamp) < 500*time.Millisecond {
			if merged, ok := tryMerge(last.Op, op); ok {
				// Recompute InverseOp for the merged op.
				// For whole-doc replaces: InverseOp.NewText = original content
				// before the sequence; we rebuild the inverse to revert
				// merged.NewText → original content.
				if rLast, ok2 := last.InverseOp.(*ReplaceRange); ok2 {
					if rMerged, ok3 := merged.(*ReplaceRange); ok3 {
						last.InverseOp = &ReplaceRange{
							Start:   rLast.Start,
							End:     EndPosition(rMerged.NewText),
							NewText: rLast.NewText,
						}
					}
				}
				last.Op = merged
				last.Timestamp = now
				last.Description = merged.Describe()
				s.content = newContent
				if s.onChange != nil {
					s.onChange(s.content, *last, DirForward)
				}
				return nil
			}
		}
	}

	s.entries = append(s.entries, entry)
	s.cursor++
	s.content = newContent

	// Capacité max : évince les entrées les plus anciennes.
	if len(s.entries) > s.capacity {
		drop := len(s.entries) - s.capacity
		s.entries = s.entries[drop:]
		s.cursor -= drop
	}

	if s.onChange != nil {
		s.onChange(s.content, entry, DirForward)
	}
	return nil
}

// Undo annule la dernière opération. Retourne l'entrée annulée et le nouveau contenu.
func (s *Stack) Undo() (string, *Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cursor == 0 {
		return "", nil, ErrNothingToUndo
	}
	entry := s.entries[s.cursor-1]
	newContent, err := entry.InverseOp.Apply(s.content)
	if err != nil {
		return "", nil, err
	}
	s.content = newContent
	s.cursor--
	if s.onChange != nil {
		s.onChange(s.content, entry, DirUndo)
	}
	return s.content, &entry, nil
}

// Redo rétablit la prochaine opération annulée.
func (s *Stack) Redo() (string, *Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cursor >= len(s.entries) {
		return "", nil, ErrNothingToRedo
	}
	entry := s.entries[s.cursor]
	newContent, err := entry.Op.Apply(s.content)
	if err != nil {
		return "", nil, err
	}
	s.content = newContent
	s.cursor++
	if s.onChange != nil {
		s.onChange(s.content, entry, DirRedo)
	}
	return s.content, &entry, nil
}

// RollbackTo annule toutes les entrées depuis le curseur jusqu'à entryID inclus.
// Utilisé par le bouton "rollback" sur un message IA dans le chat.
func (s *Stack) RollbackTo(entryID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	target := -1
	for i := s.cursor - 1; i >= 0; i-- {
		if s.entries[i].ID == entryID {
			target = i
			break
		}
	}
	if target < 0 {
		return ErrEntryNotFound
	}

	for s.cursor > target {
		entry := s.entries[s.cursor-1]
		newContent, err := entry.InverseOp.Apply(s.content)
		if err != nil {
			return err
		}
		s.content = newContent
		s.cursor--
		if s.onChange != nil {
			s.onChange(s.content, entry, DirUndo)
		}
	}
	return nil
}

// Cursor retourne la position courante dans la pile (pour les tests).
func (s *Stack) Cursor() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cursor
}

// Len retourne le nombre total d'entrées (pour les tests).
func (s *Stack) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}
