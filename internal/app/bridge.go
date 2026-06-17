package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	agentkpkg "github.com/bornholm/amatled/internal/agent"
	"github.com/bornholm/amatled/internal/editor"
	"github.com/bornholm/amatled/internal/format"
	"github.com/bornholm/amatled/internal/history"
	appkeyring "github.com/bornholm/amatled/internal/keyring"
	"github.com/bornholm/amatled/internal/render"
	"github.com/bornholm/amatled/internal/settings"
	"github.com/bornholm/amatled/internal/updater"
	"github.com/bornholm/amatled/internal/workspace"
	genaiagent "github.com/bornholm/genai/agent"
	"github.com/sqweek/dialog"
	"github.com/zserge/lorca"
)

// HandlerFunc traite un appel RPC et retourne une valeur ou une erreur.
type HandlerFunc func(paramsJSON string) (any, error)

// rpcResponse est le format de réponse JSON retourné au JS.
type rpcResponse struct {
	OK    bool   `json:"ok"`
	Value any    `json:"value,omitempty"`
	Error string `json:"error,omitempty"`
}

// Bridge gère la communication bidirectionnelle entre Go et JS.
type Bridge struct {
	ui       lorca.UI
	handlers map[string]HandlerFunc

	workspace                  *workspace.Workspace
	workspaceRoot              string
	workspaceActiveProfile     string // profil actif pour le workspace courant
	workspaceActiveRenderPreset string // préset de rendu actif pour le workspace courant
	settings               *settings.Settings
	sessions               *editor.SessionManager
	version                string

	// Cache PDF (prévisualisation)
	pdfMu    sync.Mutex
	pdfCache map[string][]byte

	// Sérialisation des dialogues natifs GTK (non thread-safe)
	dialogMu sync.Mutex

	// Agent IA
	agentMu      sync.Mutex
	agentCancel  context.CancelFunc
	agentRunning bool

	// Autoupdate
	updateMu      sync.Mutex
	pendingRelease *updater.Release

	// File watcher
	watchMu      sync.Mutex
	watchedMod   map[string]time.Time    // fileID → dernière modtime connue par l'app
	watchedPaths map[string]struct{}     // snapshot plat des chemins du workspace
	watcherStop  chan struct{}
}

func newBridge(s *settings.Settings, version string) *Bridge {
	b := &Bridge{
		handlers:   make(map[string]HandlerFunc),
		settings:   s,
		version:    version,
		watchedMod: make(map[string]time.Time),
		pdfCache:   make(map[string][]byte),
	}

	b.sessions = editor.NewSessionManager(func(fileID, content string, entry history.Entry, dir history.Direction) {
		go b.Emit("editor.contentUpdated", map[string]any{
			"fileId":      fileID,
			"content":     content,
			"source":      entry.Source,
			"entryId":     entry.ID,
			"direction":   dir,
			"description": entry.Description,
		})
	})

	// Workspace
	b.handle("workspace.selectFolder", b.handleSelectFolder)
	b.handle("workspace.open", b.handleOpenWorkspace)
	b.handle("workspace.listFiles", b.handleListFiles)
	b.handle("workspace.readFile", b.handleReadFile)
	b.handle("workspace.writeFile", b.handleWriteFile)
	b.handle("workspace.createFile", b.handleCreateFile)
	b.handle("workspace.deleteFile", b.handleDeleteFile)
	b.handle("workspace.getActiveProfile", b.handleGetActiveProfile)
	b.handle("workspace.setActiveProfile", b.handleSetActiveProfile)
	b.handle("workspace.getActiveRenderPreset", b.handleGetActiveRenderPreset)
	b.handle("workspace.setActiveRenderPreset", b.handleSetActiveRenderPreset)
	// Settings
	b.handle("settings.get", b.handleGetSettings)
	b.handle("settings.listProfiles", b.handleListProfiles)
	b.handle("settings.createProfile", b.handleCreateProfile)
	b.handle("settings.updateProfile", b.handleUpdateProfile)
	b.handle("settings.deleteProfile", b.handleDeleteProfile)
	b.handle("settings.listRenderPresets", b.handleListRenderPresets)
	b.handle("settings.createRenderPreset", b.handleCreateRenderPreset)
	b.handle("settings.updateRenderPreset", b.handleUpdateRenderPreset)
	b.handle("settings.deleteRenderPreset", b.handleDeleteRenderPreset)
	// Editor / historique
	b.handle("editor.openFile", b.handleEditorOpenFile)
	b.handle("editor.applyLocalChanges", b.handleApplyLocalChanges)
	b.handle("editor.saveFile", b.handleSaveFile)
	b.handle("history.undo", b.handleUndo)
	b.handle("history.redo", b.handleRedo)
	// Document
	b.handle("document.getActiveSection", b.handleGetActiveSection)
	b.handle("document.lockSection", b.handleLockSection)
	b.handle("document.renderPDF", b.handleDocumentRenderPDF)
	b.handle("document.exportPDF", b.handleDocumentExportPDF)
	// Chat / Agent
	b.handle("chat.sendMessage", b.handleChatSendMessage)
	b.handle("chat.cancel", b.handleChatCancel)
	// Settings LLM (legacy, conservé pour compatibilité)
	b.handle("settings.getLLM", b.handleGetLLMSettings)
	b.handle("settings.saveLLM", b.handleSaveLLMSettings)
	b.handle("settings.testLLM", b.handleTestLLMConnection)
	// Settings généraux
	b.handle("settings.saveGeneral", b.handleSaveGeneralSettings)
	// Autoupdate
	b.handle("updater.check", b.handleUpdaterCheck)
	b.handle("updater.apply", b.handleUpdaterApply)

	return b
}

// SetPendingRelease stocke la release en attente d'application (appelé par app.go).
func (b *Bridge) SetPendingRelease(release *updater.Release) {
	b.updateMu.Lock()
	b.pendingRelease = release
	b.updateMu.Unlock()
}

// Register bind le point d'entrée RPC unique dans la fenêtre lorca.
func (b *Bridge) Register(ui lorca.UI) error {
	b.ui = ui
	return ui.Bind("rpc", func(method, paramsJSON string) string {
		slog.Debug("rpc call", "method", method)
		h, ok := b.handlers[method]
		if !ok {
			return b.encode(rpcResponse{Error: fmt.Sprintf("unknown method: %s", method)})
		}
		value, err := h(paramsJSON)
		if err != nil {
			slog.Error("rpc handler error", "method", method, "err", err)
			return b.encode(rpcResponse{Error: err.Error()})
		}
		return b.encode(rpcResponse{OK: true, Value: value})
	})
}

// Emit pousse un événement vers le bus JS.
func (b *Bridge) Emit(event string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		slog.Error("bridge emit marshal error", "event", event, "err", err)
		return
	}
	b.ui.Eval(fmt.Sprintf("window.__bus && window.__bus.emit(%q, %s)", event, data))
}

func (b *Bridge) handle(method string, h HandlerFunc) {
	b.handlers[method] = h
}

func (b *Bridge) encode(resp rpcResponse) string {
	data, _ := json.Marshal(resp)
	return string(data)
}

// ─── Helpers profil actif ─────────────────────────────────────────────────────

// resolveActiveProfileName retourne le nom du profil actif :
// d'abord la valeur per-workspace, sinon le global.
func (b *Bridge) resolveActiveProfileName() string {
	if b.workspaceActiveProfile != "" {
		return b.workspaceActiveProfile
	}
	return b.settings.ActiveProfile
}

// resolveActiveProfile retourne le profil actif avec les secrets chargés depuis le keyring.
func (b *Bridge) resolveActiveProfile() settings.Profile {
	name := b.resolveActiveProfileName()
	p := b.settings.ResolveProfile(name)
	if p == nil {
		return settings.Profile{}
	}
	out := *p
	out.LLM.APIKey, _ = appkeyring.GetProfileAPIKey(p.Name)
	return out
}

// isLLMConfigured vérifie que le profil actif a une config LLM minimale.
func (b *Bridge) isLLMConfigured() bool {
	p := b.resolveActiveProfile()
	return p.LLM.Provider != "" && p.LLM.APIKey != "" && p.LLM.Model != ""
}

// profileWithSecrets retourne un profil de la liste avec ses secrets depuis le keyring.
func profileWithSecrets(p settings.Profile) settings.Profile {
	p.LLM.APIKey, _ = appkeyring.GetProfileAPIKey(p.Name)
	return p
}

// ─── Helpers préset de rendu actif ───────────────────────────────────────────

// resolveActiveRenderPresetName retourne le nom du préset de rendu actif :
// d'abord la valeur per-workspace, sinon le global.
func (b *Bridge) resolveActiveRenderPresetName() string {
	if b.workspaceActiveRenderPreset != "" {
		return b.workspaceActiveRenderPreset
	}
	return b.settings.ActiveRenderPreset
}

// resolveActiveRenderPreset retourne le préset de rendu actif avec son mot de passe depuis le keyring.
// Retourne nil si aucun préset n'est actif ou si le nom ne correspond à aucun préset connu.
func (b *Bridge) resolveActiveRenderPreset() *settings.RenderPreset {
	name := b.resolveActiveRenderPresetName()
	if name == "" {
		return nil
	}
	rp := b.settings.GetRenderPreset(name)
	if rp == nil {
		return nil
	}
	out := *rp
	out.RenderConfigPassword, _ = appkeyring.GetRenderPresetPassword(rp.Name)
	return &out
}

// renderPresetWithSecrets retourne un préset avec son mot de passe depuis le keyring.
func renderPresetWithSecrets(rp settings.RenderPreset) settings.RenderPreset {
	rp.RenderConfigPassword, _ = appkeyring.GetRenderPresetPassword(rp.Name)
	return rp
}

// currentWorkspaceConfig construit la config workspace depuis l'état courant du bridge.
func (b *Bridge) currentWorkspaceConfig() *workspace.WorkspaceConfig {
	return &workspace.WorkspaceConfig{
		ActiveProfile:      b.workspaceActiveProfile,
		ActiveRenderPreset: b.workspaceActiveRenderPreset,
	}
}

// ─── Workspace handlers ───────────────────────────────────────────────────────

func (b *Bridge) handleSelectFolder(_ string) (any, error) {
	if !b.dialogMu.TryLock() {
		return nil, fmt.Errorf("une boîte de dialogue est déjà ouverte")
	}
	defer b.dialogMu.Unlock()
	path, err := dialog.Directory().Title("Ouvrir un workspace").Browse()
	if err == dialog.ErrCancelled {
		return map[string]any{"cancelled": true}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("select folder: %w", err)
	}
	return map[string]string{"path": path}, nil
}

func (b *Bridge) handleOpenWorkspace(paramsJSON string) (any, error) {
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	ws, err := workspace.Open(params.Path)
	if err != nil {
		return nil, fmt.Errorf("open workspace: %w", err)
	}
	b.workspace = ws
	b.workspaceRoot = params.Path
	b.settings.LastWorkspace = params.Path
	if err := b.settings.Save(); err != nil {
		slog.Warn("failed to save settings", "err", err)
	}

	// Charger les préférences du workspace
	b.workspaceActiveProfile = ""
	b.workspaceActiveRenderPreset = ""
	if cfg, err := ws.LoadConfig(); err == nil {
		b.workspaceActiveProfile = cfg.ActiveProfile
		b.workspaceActiveRenderPreset = cfg.ActiveRenderPreset
	}

	files, err := ws.ListFiles()
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}
	b.startWatcher()

	return map[string]any{
		"files":               files,
		"rootPath":            ws.RootPath,
		"activeProfile":       b.resolveActiveProfileName(),
		"activeRenderPreset":  b.resolveActiveRenderPresetName(),
	}, nil
}

func (b *Bridge) handleListFiles(_ string) (any, error) {
	if b.workspace == nil {
		return map[string]any{"files": []any{}}, nil
	}
	files, err := b.workspace.ListFiles()
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}
	return map[string]any{"files": files}, nil
}

func (b *Bridge) handleReadFile(paramsJSON string) (any, error) {
	if b.workspace == nil {
		return nil, fmt.Errorf("no workspace open")
	}
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	content, err := b.workspace.ReadFile(params.Path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	return map[string]string{"content": content}, nil
}

func (b *Bridge) handleWriteFile(paramsJSON string) (any, error) {
	if b.workspace == nil {
		return nil, fmt.Errorf("no workspace open")
	}
	var params struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	if err := b.workspace.WriteFile(params.Path, params.Content); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}
	return map[string]bool{"ok": true}, nil
}

func (b *Bridge) handleCreateFile(paramsJSON string) (any, error) {
	if b.workspace == nil {
		return nil, fmt.Errorf("no workspace open")
	}
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	if err := b.workspace.CreateFile(params.Path); err != nil {
		return nil, fmt.Errorf("create file: %w", err)
	}
	if files, err := b.workspace.ListFiles(); err == nil {
		go b.Emit("workspace.treeUpdated", map[string]any{"files": files})
	}
	return map[string]bool{"ok": true}, nil
}

func (b *Bridge) handleDeleteFile(paramsJSON string) (any, error) {
	if b.workspace == nil {
		return nil, fmt.Errorf("no workspace open")
	}
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	if err := b.workspace.DeleteFile(params.Path); err != nil {
		return nil, fmt.Errorf("delete file: %w", err)
	}
	if files, err := b.workspace.ListFiles(); err == nil {
		go b.Emit("workspace.treeUpdated", map[string]any{"files": files})
	}
	return map[string]bool{"ok": true}, nil
}

func (b *Bridge) handleGetActiveProfile(_ string) (any, error) {
	return map[string]string{"name": b.resolveActiveProfileName()}, nil
}

func (b *Bridge) handleSetActiveProfile(paramsJSON string) (any, error) {
	var params struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	if b.settings.GetProfile(params.Name) == nil {
		return nil, fmt.Errorf("profil inconnu : %s", params.Name)
	}
	b.workspaceActiveProfile = params.Name
	b.settings.ActiveProfile = params.Name
	if err := b.settings.Save(); err != nil {
		slog.Warn("failed to save settings after setActiveProfile", "err", err)
	}
	if b.workspace != nil {
		if err := b.workspace.SaveConfig(b.currentWorkspaceConfig()); err != nil {
			slog.Warn("failed to save workspace config", "err", err)
		}
	}
	go b.Emit("profile.changed", map[string]string{"name": params.Name})
	return map[string]bool{"ok": true}, nil
}

func (b *Bridge) handleGetSettings(_ string) (any, error) {
	return map[string]any{
		"lastWorkspace":   b.settings.LastWorkspace,
		"normalizeOnSave": b.settings.IsNormalizeOnSave(),
		"autoUpdate":      b.settings.IsAutoUpdate(),
		"activeProfile":   b.resolveActiveProfileName(),
		"version":         b.version,
	}, nil
}

// ─── Editor / History handlers ────────────────────────────────────────────────

func (b *Bridge) handleEditorOpenFile(paramsJSON string) (any, error) {
	if b.workspace == nil {
		return nil, fmt.Errorf("no workspace open")
	}
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	content, err := b.workspace.ReadFile(params.Path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	b.sessions.Open(params.Path, content)
	b.watcherRecord(params.Path)
	return map[string]any{"content": content, "fileId": params.Path}, nil
}

func (b *Bridge) handleApplyLocalChanges(paramsJSON string) (any, error) {
	var params struct {
		FileID  string         `json:"fileId"`
		Changes []editor.Change `json:"changes"`
	}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	sess, ok := b.sessions.Get(params.FileID)
	if !ok {
		return nil, fmt.Errorf("no session for %s", params.FileID)
	}
	if err := sess.ApplyChanges(params.Changes, history.SourceHuman, ""); err != nil {
		return nil, fmt.Errorf("apply changes: %w", err)
	}
	return map[string]bool{"ok": true}, nil
}

func (b *Bridge) handleSaveFile(paramsJSON string) (any, error) {
	if b.workspace == nil {
		return nil, fmt.Errorf("no workspace open")
	}
	var params struct {
		FileID string `json:"fileId"`
	}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	sess, ok := b.sessions.Get(params.FileID)
	if !ok {
		return nil, fmt.Errorf("no session for %s", params.FileID)
	}

	content := sess.Content()
	wasNormalized := false

	if b.settings.IsNormalizeOnSave() {
		normalized, err := format.FormatMarkdown(content)
		if err != nil {
			slog.Warn("normalize on save failed, saving raw content", "fileId", params.FileID, "err", err)
		} else if normalized != content {
			content = normalized
			wasNormalized = true
			sess.SetContent(content)
		}
	}

	if err := b.workspace.WriteFile(params.FileID, content); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}
	b.watcherRecord(params.FileID)
	result := map[string]any{"ok": true}
	if wasNormalized {
		result["content"] = content
	}
	return result, nil
}

func (b *Bridge) handleUndo(paramsJSON string) (any, error) {
	var params struct {
		FileID string `json:"fileId"`
	}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	sess, ok := b.sessions.Get(params.FileID)
	if !ok {
		return nil, fmt.Errorf("no session for %s", params.FileID)
	}
	content, entry, err := sess.Undo()
	if err == history.ErrNothingToUndo {
		return map[string]any{"noop": true}, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"content": content,
		"entry":   entry,
	}, nil
}

func (b *Bridge) handleRedo(paramsJSON string) (any, error) {
	var params struct {
		FileID string `json:"fileId"`
	}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	sess, ok := b.sessions.Get(params.FileID)
	if !ok {
		return nil, fmt.Errorf("no session for %s", params.FileID)
	}
	content, entry, err := sess.Redo()
	if err == history.ErrNothingToRedo {
		return map[string]any{"noop": true}, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"content": content,
		"entry":   entry,
	}, nil
}

// ─── Document handlers ────────────────────────────────────────────────────────

func (b *Bridge) handleGetActiveSection(paramsJSON string) (any, error) {
	var params struct {
		FileID     string `json:"fileId"`
		CursorLine int    `json:"cursorLine"`
	}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	sess, ok := b.sessions.Get(params.FileID)
	if !ok {
		return nil, fmt.Errorf("no session for %s", params.FileID)
	}
	ref, err := sess.SetCursorLine(params.CursorLine)
	if err != nil {
		return nil, err
	}
	return ref, nil // nil est sérialisé en JSON null
}

func (b *Bridge) handleLockSection(paramsJSON string) (any, error) {
	var params struct {
		FileID string `json:"fileId"`
		Locked bool   `json:"locked"`
	}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	sess, ok := b.sessions.Get(params.FileID)
	if !ok {
		return nil, fmt.Errorf("no session for %s", params.FileID)
	}
	sess.LockSection(params.Locked)
	return map[string]bool{"ok": true}, nil
}

// GetCachedPDF retourne le PDF mis en cache pour un fileID, utilisé par le serveur HTTP.
func (b *Bridge) GetCachedPDF(fileID string) ([]byte, bool) {
	b.pdfMu.Lock()
	defer b.pdfMu.Unlock()
	pdf, ok := b.pdfCache[fileID]
	return pdf, ok
}

func (b *Bridge) handleDocumentRenderPDF(paramsJSON string) (any, error) {
	var params struct {
		FileID string `json:"fileId"`
	}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	sess, ok := b.sessions.Get(params.FileID)
	if !ok {
		return nil, fmt.Errorf("no session for %s", params.FileID)
	}
	var renderConfig, renderConfigUsername, renderConfigPassword string
	if rp := b.resolveActiveRenderPreset(); rp != nil {
		renderConfig = rp.RenderConfig
		renderConfigUsername = rp.RenderConfigUsername
		renderConfigPassword = rp.RenderConfigPassword
	}
	pdf, err := render.RenderPDF(
		context.Background(),
		[]byte(sess.Content()),
		sess.FileID,
		b.workspaceRoot,
		renderConfig,
		renderConfigUsername,
		renderConfigPassword,
	)
	if err != nil {
		return nil, fmt.Errorf("render pdf: %w", err)
	}
	b.pdfMu.Lock()
	b.pdfCache[params.FileID] = pdf
	b.pdfMu.Unlock()
	return map[string]bool{"ok": true}, nil
}

func (b *Bridge) handleDocumentExportPDF(paramsJSON string) (any, error) {
	var params struct {
		FileID string `json:"fileId"`
	}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	sess, ok := b.sessions.Get(params.FileID)
	if !ok {
		return nil, fmt.Errorf("no session for %s", params.FileID)
	}
	var renderConfig, renderConfigUsername, renderConfigPassword string
	if rp := b.resolveActiveRenderPreset(); rp != nil {
		renderConfig = rp.RenderConfig
		renderConfigUsername = rp.RenderConfigUsername
		renderConfigPassword = rp.RenderConfigPassword
	}
	pdf, err := render.RenderPDF(
		context.Background(),
		[]byte(sess.Content()),
		sess.FileID,
		b.workspaceRoot,
		renderConfig,
		renderConfigUsername,
		renderConfigPassword,
	)
	if err != nil {
		return nil, fmt.Errorf("render pdf: %w", err)
	}
	if !b.dialogMu.TryLock() {
		return nil, fmt.Errorf("une boîte de dialogue est déjà ouverte")
	}
	base := filepath.Base(params.FileID)
	name := base[:len(base)-len(filepath.Ext(base))]
	path, err := dialog.File().
		Title("Exporter en PDF").
		Filter("Fichiers PDF", "pdf").
		SetStartFile(name + ".pdf").
		Save()
	b.dialogMu.Unlock()
	if err == dialog.ErrCancelled {
		return map[string]any{"cancelled": true}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("save dialog: %w", err)
	}
	if !strings.HasSuffix(strings.ToLower(path), ".pdf") {
		path += ".pdf"
	}
	if err := os.WriteFile(path, pdf, 0644); err != nil {
		return nil, fmt.Errorf("write pdf: %w", err)
	}
	return map[string]bool{"ok": true}, nil
}

// ─── Chat / Agent handlers ────────────────────────────────────────────────────

func (b *Bridge) handleChatSendMessage(paramsJSON string) (any, error) {
	var params struct {
		FileID    string `json:"fileId"`
		Message   string `json:"message"`
		AIMessage string `json:"aiMessageId"`
	}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}

	b.agentMu.Lock()
	if b.agentRunning {
		b.agentMu.Unlock()
		return nil, fmt.Errorf("un agent est déjà en cours d'exécution")
	}

	sess, ok := b.sessions.Get(params.FileID)
	if !ok {
		b.agentMu.Unlock()
		return nil, fmt.Errorf("no session for %s", params.FileID)
	}

	if !b.isLLMConfigured() {
		b.agentMu.Unlock()
		return nil, fmt.Errorf("configuration LLM manquante — ouvrez les paramètres pour configurer votre provider")
	}

	activeProfile := b.resolveActiveProfile()

	ctx, cancel := context.WithCancel(context.Background())
	b.agentCancel = cancel
	b.agentRunning = true
	b.agentMu.Unlock()

	runner := agentkpkg.NewRunner(b.settings, activeProfile.LLM, activeProfile.SystemPrompt, b.workspace)

	go func() {
		defer func() {
			b.agentMu.Lock()
			b.agentRunning = false
			b.agentCancel = nil
			b.agentMu.Unlock()
		}()

		err := runner.Run(ctx, sess, params.Message, params.AIMessage, func(ev genaiagent.Event) error {
			b.Emit("agent.event", map[string]any{
				"type": ev.Type(),
				"data": ev.Data(),
			})
			return nil
		})

		if err != nil && ctx.Err() == nil {
			slog.Error("agent run error", "err", err)
			b.Emit("agent.event", map[string]any{
				"type": genaiagent.EventTypeError,
				"data": map[string]string{"message": err.Error()},
			})
		}
	}()

	return map[string]bool{"ok": true}, nil
}

func (b *Bridge) handleChatCancel(_ string) (any, error) {
	b.agentMu.Lock()
	defer b.agentMu.Unlock()
	if b.agentCancel != nil {
		b.agentCancel()
	}
	return map[string]bool{"ok": true}, nil
}

// ─── Profile handlers ─────────────────────────────────────────────────────────

func (b *Bridge) handleListProfiles(_ string) (any, error) {
	profiles := make([]settings.Profile, len(b.settings.Profiles))
	for i, p := range b.settings.Profiles {
		profiles[i] = profileWithSecrets(p)
	}
	return profiles, nil
}

func (b *Bridge) handleCreateProfile(paramsJSON string) (any, error) {
	var p settings.Profile
	if err := json.Unmarshal([]byte(paramsJSON), &p); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	if p.Name == "" {
		return nil, fmt.Errorf("le nom du profil est requis")
	}
	if b.settings.GetProfile(p.Name) != nil {
		return nil, fmt.Errorf("un profil nommé %q existe déjà", p.Name)
	}
	if err := appkeyring.SetProfileAPIKey(p.Name, p.LLM.APIKey); err != nil {
		slog.Warn("keyring set api key failed", "profile", p.Name, "err", err)
	}
	p.LLM.APIKey = ""
	b.settings.Profiles = append(b.settings.Profiles, p)
	if b.settings.ActiveProfile == "" {
		b.settings.ActiveProfile = p.Name
	}
	if err := b.settings.Save(); err != nil {
		return nil, fmt.Errorf("save settings: %w", err)
	}
	go b.Emit("profiles.updated", nil)
	return map[string]bool{"ok": true}, nil
}

func (b *Bridge) handleUpdateProfile(paramsJSON string) (any, error) {
	var p settings.Profile
	if err := json.Unmarshal([]byte(paramsJSON), &p); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	if p.Name == "" {
		return nil, fmt.Errorf("le nom du profil est requis")
	}
	found := false
	for i := range b.settings.Profiles {
		if b.settings.Profiles[i].Name == p.Name {
			if err := appkeyring.SetProfileAPIKey(p.Name, p.LLM.APIKey); err != nil {
				slog.Warn("keyring set api key failed", "profile", p.Name, "err", err)
			}
			p.LLM.APIKey = ""
			b.settings.Profiles[i] = p
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("profil inconnu : %s", p.Name)
	}
	if err := b.settings.Save(); err != nil {
		return nil, fmt.Errorf("save settings: %w", err)
	}
	return map[string]bool{"ok": true}, nil
}

func (b *Bridge) handleDeleteProfile(paramsJSON string) (any, error) {
	var params struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	if len(b.settings.Profiles) <= 1 {
		return nil, fmt.Errorf("impossible de supprimer le dernier profil")
	}
	idx := -1
	for i, p := range b.settings.Profiles {
		if p.Name == params.Name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, fmt.Errorf("profil inconnu : %s", params.Name)
	}
	if err := appkeyring.DeleteProfile(params.Name); err != nil {
		slog.Warn("keyring delete profile failed", "profile", params.Name, "err", err)
	}
	b.settings.Profiles = append(b.settings.Profiles[:idx], b.settings.Profiles[idx+1:]...)
	if b.settings.ActiveProfile == params.Name {
		b.settings.ActiveProfile = b.settings.Profiles[0].Name
	}
	if b.workspaceActiveProfile == params.Name {
		b.workspaceActiveProfile = ""
	}
	if err := b.settings.Save(); err != nil {
		return nil, fmt.Errorf("save settings: %w", err)
	}
	go b.Emit("profiles.updated", nil)
	return map[string]bool{"ok": true}, nil
}

// ─── Render preset handlers ───────────────────────────────────────────────────

func (b *Bridge) handleGetActiveRenderPreset(_ string) (any, error) {
	return map[string]string{"name": b.resolveActiveRenderPresetName()}, nil
}

func (b *Bridge) handleSetActiveRenderPreset(paramsJSON string) (any, error) {
	var params struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	if params.Name != "" && b.settings.GetRenderPreset(params.Name) == nil {
		return nil, fmt.Errorf("préset de rendu inconnu : %s", params.Name)
	}
	b.workspaceActiveRenderPreset = params.Name
	b.settings.ActiveRenderPreset = params.Name
	if err := b.settings.Save(); err != nil {
		slog.Warn("failed to save settings after setActiveRenderPreset", "err", err)
	}
	if b.workspace != nil {
		if err := b.workspace.SaveConfig(b.currentWorkspaceConfig()); err != nil {
			slog.Warn("failed to save workspace config after setActiveRenderPreset", "err", err)
		}
	}
	go b.Emit("renderPreset.changed", map[string]string{"name": params.Name})
	return map[string]bool{"ok": true}, nil
}

func (b *Bridge) handleListRenderPresets(_ string) (any, error) {
	presets := make([]settings.RenderPreset, len(b.settings.RenderPresets))
	for i, rp := range b.settings.RenderPresets {
		presets[i] = renderPresetWithSecrets(rp)
	}
	return presets, nil
}

func (b *Bridge) handleCreateRenderPreset(paramsJSON string) (any, error) {
	var rp settings.RenderPreset
	if err := json.Unmarshal([]byte(paramsJSON), &rp); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	if rp.Name == "" {
		return nil, fmt.Errorf("le nom du préset est requis")
	}
	if b.settings.GetRenderPreset(rp.Name) != nil {
		return nil, fmt.Errorf("un préset nommé %q existe déjà", rp.Name)
	}
	if err := appkeyring.SetRenderPresetPassword(rp.Name, rp.RenderConfigPassword); err != nil {
		slog.Warn("keyring set render preset password failed", "preset", rp.Name, "err", err)
	}
	rp.RenderConfigPassword = ""
	b.settings.RenderPresets = append(b.settings.RenderPresets, rp)
	if err := b.settings.Save(); err != nil {
		return nil, fmt.Errorf("save settings: %w", err)
	}
	go b.Emit("renderPresets.updated", nil)
	return map[string]bool{"ok": true}, nil
}

func (b *Bridge) handleUpdateRenderPreset(paramsJSON string) (any, error) {
	var rp settings.RenderPreset
	if err := json.Unmarshal([]byte(paramsJSON), &rp); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	if rp.Name == "" {
		return nil, fmt.Errorf("le nom du préset est requis")
	}
	found := false
	for i := range b.settings.RenderPresets {
		if b.settings.RenderPresets[i].Name == rp.Name {
			if err := appkeyring.SetRenderPresetPassword(rp.Name, rp.RenderConfigPassword); err != nil {
				slog.Warn("keyring set render preset password failed", "preset", rp.Name, "err", err)
			}
			rp.RenderConfigPassword = ""
			b.settings.RenderPresets[i] = rp
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("préset inconnu : %s", rp.Name)
	}
	if err := b.settings.Save(); err != nil {
		return nil, fmt.Errorf("save settings: %w", err)
	}
	return map[string]bool{"ok": true}, nil
}

func (b *Bridge) handleDeleteRenderPreset(paramsJSON string) (any, error) {
	var params struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	idx := -1
	for i, rp := range b.settings.RenderPresets {
		if rp.Name == params.Name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, fmt.Errorf("préset inconnu : %s", params.Name)
	}
	if err := appkeyring.DeleteRenderPreset(params.Name); err != nil {
		slog.Warn("keyring delete render preset failed", "preset", params.Name, "err", err)
	}
	b.settings.RenderPresets = append(b.settings.RenderPresets[:idx], b.settings.RenderPresets[idx+1:]...)
	if b.settings.ActiveRenderPreset == params.Name {
		b.settings.ActiveRenderPreset = ""
	}
	if b.workspaceActiveRenderPreset == params.Name {
		b.workspaceActiveRenderPreset = ""
	}
	if err := b.settings.Save(); err != nil {
		return nil, fmt.Errorf("save settings: %w", err)
	}
	go b.Emit("renderPresets.updated", nil)
	return map[string]bool{"ok": true}, nil
}

// ─── Settings LLM handlers ────────────────────────────────────────────────────

func (b *Bridge) handleGetLLMSettings(_ string) (any, error) {
	return b.settings.LLM, nil
}

func (b *Bridge) handleSaveLLMSettings(paramsJSON string) (any, error) {
	var llmSettings settings.LLMSettings
	if err := json.Unmarshal([]byte(paramsJSON), &llmSettings); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	b.settings.LLM = llmSettings
	if err := b.settings.Save(); err != nil {
		return nil, fmt.Errorf("save settings: %w", err)
	}
	return map[string]bool{"ok": true}, nil
}

func (b *Bridge) handleSaveGeneralSettings(paramsJSON string) (any, error) {
	var params struct {
		NormalizeOnSave bool `json:"normalizeOnSave"`
		AutoUpdate      bool `json:"autoUpdate"`
	}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	b.settings.NormalizeOnSave = &params.NormalizeOnSave
	b.settings.AutoUpdate = &params.AutoUpdate
	if err := b.settings.Save(); err != nil {
		return nil, fmt.Errorf("save settings: %w", err)
	}
	return map[string]bool{"ok": true}, nil
}

// ─── Updater handlers ─────────────────────────────────────────────────────────

func (b *Bridge) handleUpdaterCheck(_ string) (any, error) {
	release, err := updater.Check(context.Background(), b.version)
	if err != nil {
		return nil, fmt.Errorf("update check: %w", err)
	}
	if release == nil {
		return map[string]any{"upToDate": true}, nil
	}
	b.SetPendingRelease(release)
	return map[string]any{"upToDate": false, "version": release.Version()}, nil
}

func (b *Bridge) handleUpdaterApply(_ string) (any, error) {
	b.updateMu.Lock()
	release := b.pendingRelease
	b.updateMu.Unlock()

	if release == nil {
		return nil, fmt.Errorf("aucune mise à jour en attente — lancez updater.check d'abord")
	}
	if err := updater.Apply(context.Background(), release); err != nil {
		return nil, fmt.Errorf("apply update: %w", err)
	}
	b.updateMu.Lock()
	b.pendingRelease = nil
	b.updateMu.Unlock()

	go b.Emit("updater.updateApplied", map[string]any{"version": release.Version()})
	return map[string]bool{"ok": true}, nil
}

func (b *Bridge) handleTestLLMConnection(_ string) (any, error) {
	if !b.isLLMConfigured() {
		return nil, fmt.Errorf("configuration LLM incomplète")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runner := agentkpkg.NewRunner(b.settings, b.resolveActiveProfile().LLM, "", b.workspace)
	if err := runner.TestConnection(ctx); err != nil {
		return nil, fmt.Errorf("connexion échouée : %w", err)
	}
	return map[string]bool{"ok": true}, nil
}

// ─── File watcher (polling) ───────────────────────────────────────────────────

// watcherRecord enregistre la modtime actuelle du fichier.
func (b *Bridge) watcherRecord(fileID string) {
	if b.workspace == nil {
		return
	}
	abs, err := b.workspace.AbsPath(fileID)
	if err != nil {
		return
	}
	if info, err := os.Stat(abs); err == nil {
		b.watchMu.Lock()
		b.watchedMod[fileID] = info.ModTime()
		b.watchMu.Unlock()
	}
}

// startWatcher démarre le polling d'un watcher pour les fichiers ouverts et l'arborescence.
func (b *Bridge) startWatcher() {
	b.watchMu.Lock()
	if b.watcherStop != nil {
		close(b.watcherStop)
	}
	stop := make(chan struct{})
	b.watcherStop = stop
	if b.workspace != nil {
		if files, err := b.workspace.ListFiles(); err == nil {
			b.watchedPaths = flattenPaths(files)
		}
	}
	b.watchMu.Unlock()

	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				b.watcherPoll()
			}
		}
	}()
}

// watcherPoll vérifie si un fichier ouvert a été modifié et si l'arborescence a changé.
func (b *Bridge) watcherPoll() {
	if b.workspace == nil {
		return
	}

	// ── Contenu des fichiers ouverts ──────────────────────────────────────────
	b.watchMu.Lock()
	snapshot := make(map[string]time.Time, len(b.watchedMod))
	maps.Copy(snapshot, b.watchedMod)
	b.watchMu.Unlock()

	for fileID, knownMod := range snapshot {
		abs, err := b.workspace.AbsPath(fileID)
		if err != nil {
			continue
		}
		info, err := os.Stat(abs)
		if err != nil {
			continue
		}
		if info.ModTime().After(knownMod) {
			slog.Debug("file changed on disk", "fileId", fileID)
			b.watchMu.Lock()
			b.watchedMod[fileID] = info.ModTime()
			b.watchMu.Unlock()
			b.Emit("editor.fileChangedOnDisk", map[string]string{"fileId": fileID})
		}
	}

	// ── Arborescence du workspace ─────────────────────────────────────────────
	files, err := b.workspace.ListFiles()
	if err != nil {
		return
	}
	current := flattenPaths(files)

	b.watchMu.Lock()
	changed := !pathSetsEqual(b.watchedPaths, current)
	if changed {
		b.watchedPaths = current
	}
	b.watchMu.Unlock()

	if changed {
		slog.Debug("workspace tree changed")
		b.Emit("workspace.treeUpdated", map[string]any{"files": files})
	}
}

// flattenPaths extrait un ensemble plat de chemins depuis une arborescence FileEntry.
func flattenPaths(entries []workspace.FileEntry) map[string]struct{} {
	set := make(map[string]struct{})
	var walk func([]workspace.FileEntry)
	walk = func(es []workspace.FileEntry) {
		for _, e := range es {
			set[e.Path] = struct{}{}
			if e.IsDir {
				walk(e.Children)
			}
		}
	}
	walk(entries)
	return set
}

// pathSetsEqual compare deux ensembles de chemins.
func pathSetsEqual(a, b map[string]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}
