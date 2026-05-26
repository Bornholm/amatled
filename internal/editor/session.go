package editor

import (
	"fmt"
	"sync"

	"github.com/bornholm/amatled/internal/document"
	"github.com/bornholm/amatled/internal/history"
)

// Change décrit une modification atomique envoyée par le frontend.
// Correspond au format CM6 : remplacement d'une plage par un nouveau texte.
type Change struct {
	From   history.Position `json:"from"`
	To     history.Position `json:"to"`
	Insert string           `json:"insert"`
}

// OnChangeFunc est appelée par le Stack lorsque le contenu change.
type OnChangeFunc func(fileID, content string, entry history.Entry, dir history.Direction)

// Session représente une session d'édition ouverte pour un fichier.
type Session struct {
	FileID        string
	mu            sync.RWMutex
	stack         *history.Stack
	activeSection *document.SectionRef
	sectionLocked bool
	onChange      OnChangeFunc
}

// newSession crée une session d'édition initialisée avec le contenu du fichier.
func newSession(fileID, initialContent string, onChange OnChangeFunc) *Session {
	s := &Session{
		FileID:   fileID,
		onChange: onChange,
	}
	s.stack = history.New(initialContent, func(content string, entry history.Entry, dir history.Direction) {
		if onChange != nil {
			onChange(fileID, content, entry, dir)
		}
	})
	return s
}

// Content retourne le contenu courant (source of truth).
func (s *Session) Content() string {
	return s.stack.Content()
}

// SetContent remplace le contenu entier et l'enregistre dans l'historique.
func (s *Session) SetContent(newContent string) {
	current := s.stack.Content()
	if current == newContent {
		return
	}
	op := &history.ReplaceRange{
		Start:   history.Position{Line: 0, Column: 0},
		End:     history.EndPosition(current),
		NewText: newContent,
	}
	_ = s.stack.Push(op, history.SourceHuman, "")
}

// ApplyChanges applique une liste de changements provenant du frontend (SourceHuman).
// Plusieurs changements dans le même appel sont appliqués dans l'ordre et coalescés.
func (s *Session) ApplyChanges(changes []Change, src history.Source, aiMsgID string) error {
	if len(changes) == 0 {
		return nil
	}
	for _, c := range changes {
		op := &history.ReplaceRange{
			Start:   c.From,
			End:     c.To,
			NewText: c.Insert,
		}
		if err := s.stack.Push(op, src, aiMsgID); err != nil {
			return fmt.Errorf("push change: %w", err)
		}
	}
	return nil
}

// Undo annule la dernière opération et retourne le nouveau contenu.
func (s *Session) Undo() (string, *history.Entry, error) {
	return s.stack.Undo()
}

// Redo rétablit la prochaine opération annulée.
func (s *Session) Redo() (string, *history.Entry, error) {
	return s.stack.Redo()
}

// RollbackTo annule toutes les entrées jusqu'à entryID inclus.
func (s *Session) RollbackTo(entryID string) error {
	return s.stack.RollbackTo(entryID)
}

// SetCursorLine recalcule la section active pour la ligne de curseur donnée.
// Ne met pas à jour si la section est verrouillée.
func (s *Session) SetCursorLine(cursorLine int) (*document.SectionRef, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sectionLocked {
		return s.activeSection, nil
	}
	ref, err := document.ActiveSection(s.stack.Content(), cursorLine)
	if err != nil {
		return nil, err
	}
	s.activeSection = ref
	return ref, nil
}

// ActiveSection retourne la section active courante (sans recalcul).
func (s *Session) ActiveSection() *document.SectionRef {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeSection
}

// LockSection verrouille ou déverrouille la section active.
func (s *Session) LockSection(locked bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sectionLocked = locked
}

// SectionLocked retourne l'état du verrou.
func (s *Session) SectionLocked() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sectionLocked
}

// ─── SessionManager ───────────────────────────────────────────────────────────

// SessionManager gère l'ensemble des sessions d'édition ouvertes.
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	onChange OnChangeFunc
}

// NewSessionManager crée un gestionnaire de sessions.
func NewSessionManager(onChange OnChangeFunc) *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*Session),
		onChange: onChange,
	}
}

// Open ouvre ou remplace une session pour le fichier fileID.
func (m *SessionManager) Open(fileID, content string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := newSession(fileID, content, m.onChange)
	m.sessions[fileID] = s
	return s
}

// Get retourne la session existante pour fileID.
func (m *SessionManager) Get(fileID string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[fileID]
	return s, ok
}

// Close ferme la session pour fileID.
func (m *SessionManager) Close(fileID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, fileID)
}
