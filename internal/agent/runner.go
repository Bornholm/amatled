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
	"github.com/bornholm/amatled/internal/workspace"
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
	settings     *settings.Settings
	llmSettings  settings.LLMSettings // profil actif résolu (avec APIKey depuis keyring)
	systemPrompt string               // prompt système personnalisé du profil (peut être vide)
	workspace    *workspace.Workspace
}

// NewRunner crée un Runner à partir des settings courants et du profil résolu.
// llm doit inclure l'APIKey récupérée depuis le keyring.
// ws peut être nil si aucun workspace n'est ouvert.
func NewRunner(s *settings.Settings, llm settings.LLMSettings, systemPrompt string, ws *workspace.Workspace) *Runner {
	return &Runner{settings: s, llmSettings: llm, systemPrompt: systemPrompt, workspace: ws}
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

	// Construction de l'index workspace (fraîcheur garantie à chaque run)
	var wsIdx *document.WorkspaceIndex
	if r.workspace != nil {
		wsIdx, _ = document.BuildWorkspaceIndex(r.workspace)
	}

	// Contexte de session pour les outils
	sc := &tools.SessionContext{
		Session:   sess,
		AIMessage: aiMsgID,
		Workspace: r.workspace,
	}
	toolCtx := tools.WithSessionContext(ctx, sc)

	systemPrompt := buildSystemPrompt(sess, wsIdx, r.systemPrompt)

	maxIter := r.llmSettings.MaxIterations
	if maxIter <= 0 {
		maxIter = 20
	}
	maxTokens := r.llmSettings.MaxTokens
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

	// On wrap l'emit pour détecter un complete vide après des appels d'outils
	// et déclencher un appel LLM de synthèse dans ce cas.
	var hadToolCalls bool
	safeEmit := func(ev genaiagent.Event) error {
		if ev.Type() == genaiagent.EventTypeToolCallStart {
			hadToolCalls = true
		}
		if ev.Type() == genaiagent.EventTypeComplete && hadToolCalls {
			data, _ := ev.Data().(*genaiagent.CompleteData)
			if data != nil && data.Message == "" {
				summary := requestCompletionSummary(ctx, client, userMsg)
				if summary != "" {
					_ = emit(genaiagent.NewEvent(genaiagent.EventTypeTextDelta, &genaiagent.TextDeltaData{Delta: summary}))
					return emit(genaiagent.NewEvent(genaiagent.EventTypeComplete, &genaiagent.CompleteData{Message: summary}))
				}
			}
		}
		return emit(ev)
	}

	if err := agentRunner.Run(toolCtx, input, safeEmit); err != nil {
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
	s := r.llmSettings
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
		tools.NewListWorkspaceSectionsTool(),
		tools.NewReadWorkspaceSectionTool(),
	}
}

// buildSystemPrompt construit le prompt système avec ToC, section active et index workspace.
// customPrompt est le prompt personnalisé du profil actif ; s'il est non vide, il est
// préfixé avant les instructions techniques.
func buildSystemPrompt(sess *editor.Session, wsIdx *document.WorkspaceIndex, customPrompt string) string {
	content := sess.Content()
	toc := generateTOC(content)

	var sb strings.Builder
	if customPrompt != "" {
		sb.WriteString(customPrompt)
		sb.WriteString("\n\n")
	}
	sb.WriteString(`Tu es un rédacteur et éditeur technique expert. Tu assistes un auteur à rédiger et améliorer un document Markdown.

Fichier actif : `)
	sb.WriteString(sess.FileID)
	sb.WriteString("\n\nTable des matières du document actif :\n")
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

	if wsIdx != nil && len(wsIdx.Files) > 0 {
		sb.WriteString(fmt.Sprintf("\n\nWorkspace (%d fichier(s) disponible(s)) :\n", len(wsIdx.Files)))
		sb.WriteString(document.FormatWorkspaceIndex(wsIdx))
		sb.WriteString("\nUtilise list_workspace_sections pour voir le détail complet, puis read_workspace_section pour lire une section précise d'un autre fichier.")
	}

	sb.WriteString(`

Tu disposes d'outils pour lire et modifier le document. Utilise-les avec précision en préservant la structure Markdown, le ton et le style d'écriture existants.
Lorsque tu modifies du contenu, conserve le même niveau de heading de la section originale.
Réponds en français sauf si l'utilisateur écrit dans une autre langue.

RÈGLES IMPÉRATIVES :
- Agis directement et de façon autonome. Ne demande JAMAIS de confirmation avant d'effectuer les modifications demandées.
- Après avoir utilisé des outils, produis TOUJOURS un message final non vide résumant ce que tu as fait.
- Ne propose jamais de plan à valider : exécute immédiatement ce qui est demandé.`)

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

// requestCompletionSummary fait un appel LLM direct (sans boucle d'outils) pour obtenir
// un résumé en 1-2 phrases de ce que l'agent vient d'effectuer. Utilisé uniquement
// quand la boucle se termine avec un message vide après des appels d'outils.
func requestCompletionSummary(ctx context.Context, client llm.ChatCompletionClient, userMsg string) string {
	messages := []llm.Message{
		llm.NewMessage(llm.RoleSystem, "Tu es un assistant d'édition. Réponds en français, en 1 à 2 phrases concises."),
		llm.NewMessage(llm.RoleUser, fmt.Sprintf(
			"J'ai demandé : « %s ». Tu as effectué les modifications demandées dans le document. Résume brièvement ce que tu as fait.",
			userMsg,
		)),
	}
	res, err := client.ChatCompletion(ctx, llm.WithMessages(messages...))
	if err != nil || res.Message() == nil {
		return ""
	}
	return res.Message().Content()
}

// SectionRef est un alias local pour éviter l'import circulaire dans les helpers.
type SectionRef = document.SectionRef
