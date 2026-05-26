package tools

import (
	"context"

	"github.com/bornholm/amatled/internal/editor"
)

type ctxKey struct{}

// SessionContext transporte la session d'édition active dans le contexte Go.
type SessionContext struct {
	Session   *editor.Session
	AIMessage string // ID du message Chat déclencheur
}

func WithSessionContext(ctx context.Context, sc *SessionContext) context.Context {
	return context.WithValue(ctx, ctxKey{}, sc)
}

func SessionContextFrom(ctx context.Context) (*SessionContext, bool) {
	sc, ok := ctx.Value(ctxKey{}).(*SessionContext)
	return sc, ok
}
