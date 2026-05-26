package agent

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"strings"

	"github.com/bornholm/amatled/internal/agent/tools"
	"github.com/bornholm/amatled/internal/document"
	"github.com/bornholm/amatled/internal/editor"
	"github.com/bornholm/amatled/internal/format"
	"github.com/bornholm/amatled/internal/history"
	"github.com/bornholm/amatled/internal/settings"
	genaiagent "github.com/bornholm/genai/agent"
	"github.com/bornholm/genai/agent/loop"
	"github.com/bornholm/genai/llm"
	"github.com/bornholm/genai/llm/provider"
	openaiProvider "github.com/bornholm/genai/llm/provider/openai"

	// Enregistre tous les providers disponibles
	_ "github.com/bornholm/genai/llm/provider/all"
)

// Runner exécute l'agent IA sur une session d'édition.
type Runner struct {
	settings *settings.Settings
}

// NewRunner crée un Runner à partir des settings courants.
func NewRunner(s *settings.Settings) *Runner {
	return &Runner{settings: s}
}

// Run démarre l'agent pour un message utilisateur donné et émet les événements au fil de l'eau.
// ctx doit être annulable pour permettre l'interruption de l'agent.
func (r *Runner) Run(
	ctx context.Context,
	sess *editor.Session,
	userMsg string,
	aiMsgID string,
	emit genaiagent.EmitFunc,
) error {
	client, err := r.createClient(ctx)
	if err != nil {
		return fmt.Errorf("create llm client: %w", err)
	}

	agentTools := r.buildTools()

	// Contexte de session pour les outils
	sc := &tools.SessionContext{
		Session:   sess,
		AIMessage: aiMsgID,
	}
	toolCtx := tools.WithSessionContext(ctx, sc)

	systemPrompt := buildSystemPrompt(sess)

	maxIter := r.settings.LLM.MaxIterations
	if maxIter <= 0 {
		maxIter = 20
	}
	maxTokens := r.settings.LLM.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 80_000
	}

	handler, err := loop.NewHandler(
		loop.WithClient(client),
		loop.WithSystemPrompt(systemPrompt),
		loop.WithTools(agentTools...),
		loop.WithMaxIterations(maxIter),
		loop.WithMaxTokens(maxTokens),
		loop.WithForcePlanningStep(false),
	)
	if err != nil {
		return fmt.Errorf("create loop handler: %w", err)
	}

	agentRunner := genaiagent.NewRunner(handler)
	input := genaiagent.NewInput(userMsg)

	// On wrap l'emit pour injecter le contexte de session dans le ctx des tools
	if err := agentRunner.Run(toolCtx, input, emit); err != nil {
		return err
	}

	// Applique une passe de normalisation Markdown après chaque run agent.
	applyMarkdownFormat(sess, aiMsgID)
	return nil
}

// applyMarkdownFormat formate le contenu courant de la session via le renderer amatl.
// Si le résultat est identique, aucune entrée n'est ajoutée à l'historique.
func applyMarkdownFormat(sess *editor.Session, aiMsgID string) {
	content := sess.Content()
	formatted, err := format.FormatMarkdown(content)
	if err != nil {
		slog.Warn("markdown format failed", "err", err)
		return
	}
	if formatted == content {
		return
	}

	lines := strings.Split(content, "\n")
	lastLine := lines[len(lines)-1]
	endPos := history.Position{
		Line:   len(lines) - 1,
		Column: len([]rune(lastLine)),
	}

	change := editor.Change{
		From:   history.Position{Line: 0, Column: 0},
		To:     endPos,
		Insert: formatted,
	}
	if err := sess.ApplyChanges([]editor.Change{change}, history.SourceAgent, aiMsgID); err != nil {
		slog.Warn("apply markdown format failed", "err", err)
	}
}

func (r *Runner) createClient(ctx context.Context) (llm.ChatCompletionClient, error) {
	s := r.settings.LLM
	provName := provider.Name(s.Provider)
	if provName == "" {
		provName = openaiProvider.Name
	}

	provOpts := provider.NewChatCompletionProviderOptions(provName)
	if provOpts == nil {
		return nil, fmt.Errorf("unknown provider: %s", provName)
	}

	// Peuple CommonOptions via réflexion (tous les providers l'embarquent).
	v := reflect.ValueOf(provOpts).Elem()
	if f := v.FieldByName("CommonOptions"); f.IsValid() && f.CanSet() {
		f.Set(reflect.ValueOf(provider.CommonOptions{
			BaseURL: s.BaseURL,
			APIKey:  s.APIKey,
			Model:   s.Model,
		}))
	}

	client, err := provider.Create(ctx, func(o *provider.Options) error {
		o.ChatCompletion = &provider.ResolvedClientOptions{
			Provider: provName,
			Specific: provOpts,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (r *Runner) buildTools() []llm.Tool {
	return []llm.Tool{
		tools.NewReadSectionTool(),
		tools.NewReadDocumentTool(),
		tools.NewReplaceSectionTool(),
		tools.NewInsertBeforeSectionTool(),
		tools.NewInsertAfterSectionTool(),
		tools.NewListSectionsTool(),
		tools.NewGetSectionByTitleTool(),
	}
}

// buildSystemPrompt construit le prompt système avec ToC et section active.
func buildSystemPrompt(sess *editor.Session) string {
	content := sess.Content()
	toc := generateTOC(content)

	var sb strings.Builder
	sb.WriteString(`Tu es un rédacteur et éditeur technique expert. Tu assistes un auteur à rédiger et améliorer un document Markdown.

Fichier actif : `)
	sb.WriteString(sess.FileID)
	sb.WriteString("\n\nTable des matières du document :\n")
	sb.WriteString(toc)

	section := sess.ActiveSection()
	if section != nil {
		sb.WriteString("\n\nSection actuellement sélectionnée :\n")
		sb.WriteString(fmt.Sprintf("Heading : %s %s\n", strings.Repeat("#", section.HeadingLevel), section.HeadingTitle))
		sb.WriteString(fmt.Sprintf("Lignes : %d à %d\n", section.StartLine, section.EndLine))
		sb.WriteString("\nContenu :\n")
		sb.WriteString(section.RawContent)
	} else {
		sb.WriteString("\n\nAucune section spécifique n'est sélectionnée. Tu peux lire et modifier n'importe quelle partie du document.")
	}

	sb.WriteString(`

Tu disposes d'outils pour lire et modifier le document. Utilise-les avec précision en préservant la structure Markdown, le ton et le style d'écriture existants.
Lorsque tu modifies du contenu, conserve le même niveau de heading de la section originale.
Réponds en français sauf si l'utilisateur écrit dans une autre langue.`)

	return sb.String()
}

// generateTOC extrait la table des matières d'un document Markdown.
func generateTOC(content string) string {
	lines := strings.Split(content, "\n")
	var sb strings.Builder
	for _, line := range lines {
		if !strings.HasPrefix(line, "#") {
			continue
		}
		level := 0
		for level < len(line) && line[level] == '#' {
			level++
		}
		if level == 0 {
			continue
		}
		title := strings.TrimSpace(line[level:])
		indent := strings.Repeat("  ", level-1)
		sb.WriteString(indent + "- " + title + "\n")
	}
	if sb.Len() == 0 {
		return "(document sans headings)\n"
	}
	return sb.String()
}

// TestConnection vérifie que le client LLM est joignable avec un message minimal.
func (r *Runner) TestConnection(ctx context.Context) error {
	client, err := r.createClient(ctx)
	if err != nil {
		return err
	}
	_, err = client.ChatCompletion(ctx,
		llm.WithMessages(llm.NewMessage(llm.RoleUser, "ping")),
	)
	return err
}

// SectionRef est un alias local pour éviter l'import circulaire dans les helpers.
type SectionRef = document.SectionRef
