import { rpc, bus, waitForBridge } from "./bridge";
import { FileTree, FileEntry } from "./tree/tree";
import { TabManager } from "./tabs/tabs";
import { Editor } from "./editor/editor";
import { EDITOR_THEMES, EditorThemeName } from "./editor/theme";
import { Preview } from "./preview/preview";
import { Chat } from "./chat/chat";
import { SettingsModal } from "./settings/settings";
import { toast } from "./toast/toast";
import { ValidationPanel } from "./validation/validation";

interface Settings {
  lastWorkspace: string;
}

interface WorkspaceResult {
  files: FileEntry[];
  rootPath: string;
  initialFile?: string;
  initialFileContent?: { content: string; fileId: string };
}

interface OpenFileResult {
  content: string;
  fileId: string;
}

interface SectionRef {
  headingLevel: number;
  headingTitle: string;
  startLine: number;
  endLine: number;
  rawContent: string;
}

interface StagedContentResult {
  original: string;
  modified: string;
}

// ── État du panel de validation ───────────────────────────────────────────────
let validationViewBeforeOpen: "source" | "preview" = "source";
let isClosingValidation = false;
let validationPanel: ValidationPanel;

// ── Éléments DOM ──────────────────────────────────────────────────────────────
const btnSettings = document.getElementById("btn-settings")!;
const mainLayout = document.getElementById("main-layout")!;
const renderPresetBar = document.getElementById("render-preset-bar")!;
const profileBar = document.getElementById("profile-bar")!;
const treeFiles = document.getElementById("tree-files")!;
const tabsBar = document.getElementById("tabs-bar")!;
const editorContent = document.getElementById("editor-content")!;
const editorEmpty = document.getElementById("editor-empty")!;
const previewContainer = document.getElementById("preview-container")!;
const previewFrame = document.getElementById("preview-frame") as HTMLIFrameElement;
const previewSpinner = previewContainer.querySelector<HTMLElement>(".preview-spinner")!;
const validationPanelEl = document.getElementById("validation-panel")!;
const sectionNameEl = document.getElementById("section-name")!;
const sectionIndicator = document.getElementById("section-indicator")!;
const chatMessagesEl = document.getElementById("chat-messages")!;
const chatInputEl = document.getElementById("chat-input") as HTMLTextAreaElement;
const chatSendBtn = document.getElementById("chat-send") as HTMLButtonElement;
const chatCancelBtn = document.getElementById("chat-cancel") as HTMLButtonElement;
const fileChangedBanner = document.getElementById("file-changed-banner")!;
const fileChangedMsg = document.getElementById("file-changed-msg")!;
const fileChangedReload = document.getElementById("file-changed-reload") as HTMLButtonElement;
const fileChangedDismiss = document.getElementById("file-changed-dismiss") as HTMLButtonElement;

// ── Menu items ────────────────────────────────────────────────────────────────
const menuOpenWorkspace = document.getElementById("menu-open-workspace") as HTMLButtonElement;
const menuExportPdf = document.getElementById("menu-export-pdf") as HTMLButtonElement;
const menuWrap = document.getElementById("menu-wrap") as HTMLButtonElement;
const menuToggleChat = document.getElementById("menu-toggle-chat") as HTMLButtonElement;
const menuNewChat = document.getElementById("menu-new-chat") as HTMLButtonElement;
const menuThemeButtons = Array.from(
  document.querySelectorAll<HTMLButtonElement>("[data-theme]")
);


// ── Logique de la barre de menu ───────────────────────────────────────────────
function setupMenubar(): void {
  document.querySelectorAll<HTMLElement>(".menu-item").forEach((item) => {
    const btn = item.querySelector<HTMLButtonElement>(".menu-btn");
    if (!btn) return;
    btn.addEventListener("click", (e) => {
      e.stopPropagation();
      const wasOpen = item.classList.contains("open");
      closeAllMenus();
      if (!wasOpen) item.classList.add("open");
    });
  });

  document.addEventListener("click", closeAllMenus);

  document.querySelectorAll<HTMLElement>(".menu-dropdown").forEach((dd) => {
    dd.addEventListener("click", (e) => {
      const action = (e.target as HTMLElement).closest<HTMLButtonElement>(".menu-action");
      if (action && !action.disabled) closeAllMenus();
    });
  });
}

function closeAllMenus(): void {
  document.querySelectorAll<HTMLElement>(".menu-item.open").forEach((el) => el.classList.remove("open"));
}

setupMenubar();

// ── Sélecteur de profil (sidebar) ────────────────────────────────────────────
const profileSelectEl = document.createElement("select");
profileSelectEl.className = "profile-selector";
profileSelectEl.title = "Profil actif pour ce workspace";
profileBar.appendChild(profileSelectEl);

async function refreshProfileSelector(activeProfileName?: string): Promise<void> {
  try {
    const [profiles, active] = await Promise.all([
      rpc<{ name: string }[]>("settings.listProfiles", {}),
      activeProfileName !== undefined
        ? Promise.resolve({ name: activeProfileName })
        : rpc<{ name: string }>("workspace.getActiveProfile", {}),
    ]);
    profileSelectEl.innerHTML = "";
    for (const p of profiles) {
      const opt = document.createElement("option");
      opt.value = p.name;
      opt.textContent = p.name;
      profileSelectEl.appendChild(opt);
    }
    profileSelectEl.value = active.name;
    profileBar.style.display = profiles.length > 0 ? "" : "none";
  } catch {
    profileBar.style.display = "none";
  }
}

profileSelectEl.addEventListener("change", async () => {
  try {
    await rpc("workspace.setActiveProfile", { name: profileSelectEl.value });
  } catch (err) {
    toast.show("Impossible de changer de profil : " + String(err), "error");
  }
});

// ── Sélecteur de préset de rendu ─────────────────────────────────────────────
const renderPresetSelectEl = document.createElement("select");
renderPresetSelectEl.className = "profile-selector render-preset-selector";
renderPresetSelectEl.title = "Configuration de rendu active";
renderPresetBar.appendChild(renderPresetSelectEl);

async function refreshRenderPresetSelector(activePresetName?: string): Promise<void> {
  try {
    const [presets, active] = await Promise.all([
      rpc<{ name: string }[]>("settings.listRenderPresets", {}),
      activePresetName !== undefined
        ? Promise.resolve({ name: activePresetName })
        : rpc<{ name: string }>("workspace.getActiveRenderPreset", {}),
    ]);
    renderPresetSelectEl.innerHTML = "";
    const noneOpt = document.createElement("option");
    noneOpt.value = "";
    noneOpt.textContent = "— Aucun rendu —";
    renderPresetSelectEl.appendChild(noneOpt);
    for (const p of presets) {
      const opt = document.createElement("option");
      opt.value = p.name;
      opt.textContent = p.name;
      renderPresetSelectEl.appendChild(opt);
    }
    renderPresetSelectEl.value = active.name ?? "";
    renderPresetBar.style.display = "";
  } catch {
    renderPresetBar.style.display = "none";
  }
}

renderPresetSelectEl.addEventListener("change", async () => {
  try {
    await rpc("workspace.setActiveRenderPreset", { name: renderPresetSelectEl.value });
  } catch (err) {
    toast.show("Impossible de changer de préset de rendu : " + String(err), "error");
  }
});

// ── Preview & toggle Source/Mise en forme ──────────────────────────────────────
const preview = new Preview(previewFrame, previewSpinner);
let currentView: "source" | "preview" = "source";

async function switchView(view: "source" | "preview"): Promise<void> {
  if (view === currentView) return;
  currentView = view;
  viewBtns.forEach((b) => b.classList.toggle("active", b.dataset.view === view));
  menuExportPdf.disabled = !tabs.getActive();
  if (view === "preview") {
    editorContent.classList.add("hidden");
    previewContainer.classList.remove("hidden");
    const activeTab = tabs.getActive();
    if (activeTab) {
      try {
        await preview.render(activeTab.fileId);
      } catch (err) {
        toast.show("Erreur de rendu : " + String(err), "error");
      }
    }
  } else {
    previewContainer.classList.add("hidden");
    editorContent.classList.remove("hidden");
    preview.clear();
  }
}

const viewBtns = document.querySelectorAll<HTMLButtonElement>(".view-btn");
viewBtns.forEach((btn) => {
  btn.addEventListener("click", () => switchView(btn.dataset.view as "source" | "preview"));
});

// ── Bannière fichier modifié ──────────────────────────────────────────────────
let pendingReloadFileId: string | null = null;

function showFileChangedBanner(fileId: string): void {
  pendingReloadFileId = fileId;
  fileChangedMsg.textContent = `Le fichier "${fileId}" a changé sur le disque.`;
  fileChangedBanner.classList.remove("hidden");
}

function hideFileChangedBanner(): void {
  fileChangedBanner.classList.add("hidden");
  pendingReloadFileId = null;
}

fileChangedReload.addEventListener("click", async () => {
  const fileId = pendingReloadFileId;
  hideFileChangedBanner();
  if (!fileId) return;
  try {
    const result = await rpc<{ content: string; fileId: string }>("editor.openFile", { path: fileId });
    const activeTab = tabs.getActive();
    if (activeTab?.fileId === fileId) {
      editor.setContent(result.content);
      tabs.updateContent(fileId, result.content);
    }
    toast.show("Fichier rechargé.", "success", 2500);
  } catch (err) {
    toast.show("Erreur lors du rechargement : " + String(err), "error");
  }
});

fileChangedDismiss.addEventListener("click", hideFileChangedBanner);

// ── Panel de validation ─────────────────────────────────────────────────────
async function openValidationPanel(): Promise<void> {
  if (validationPanelEl.classList.contains("hidden")) {
    validationViewBeforeOpen = currentView;
  }

  const activeTab = tabs.getActive();
  if (!activeTab) {
    toast.show("Aucun fichier ouvert.", "info", 2000);
    return;
  }

  // Flusher les changements CM6 en attente avant de récupérer le contenu.
  await editor.flushSync().catch(() => {});

  try {
    const result = await rpc<StagedContentResult>("editor.stageChanges", {
      fileId: activeTab.fileId,
    });

    // Si le contenu n'a pas changé, valider directement sans ouvrir le panel.
    if (result.original === result.modified) {
      await forceSaveCurrentFile();
      return;
    }

    // Masquer l'éditeur / preview et afficher le panel de validation.
    editorContent.classList.add("hidden");
    previewContainer.classList.add("hidden");
    editorEmpty.classList.add("hidden");
    validationPanelEl.classList.remove("hidden");

    validationPanel.open(activeTab.fileId, result.original, result.modified);
  } catch (err) {
    toast.show("Impossible d'ouvrir le panel de validation : " + String(err), "error");
  }
}

async function forceSaveCurrentFile(): Promise<void> {
  const activeTab = tabs.getActive();
  if (!activeTab) return;
  try {
    await editor.save();
    tabs.setDirty(activeTab.fileId, false);
    toast.show("Fichier sauvegardé.", "success", 2000);
  } catch (err) {
    toast.show("Erreur de sauvegarde : " + String(err), "error");
  }
}

async function confirmDiscardChanges(): Promise<boolean> {
  return confirm("Des modifications non validées seront perdues. Continuer ?");
}

async function closeValidationPanel(restore = true): Promise<void> {
  if (isClosingValidation) return;
  isClosingValidation = true;

  validationPanel.close();
  validationPanelEl.classList.add("hidden");

  if (restore) {
    // Restaurer le mode d'affichage précédent
    if (validationViewBeforeOpen === "preview") {
      editorContent.classList.add("hidden");
      previewContainer.classList.remove("hidden");
      const activeTab = tabs.getActive();
      if (activeTab) {
        preview.render(activeTab.fileId).catch(console.error);
      }
    } else {
      previewContainer.classList.add("hidden");
      editorContent.classList.remove("hidden");
      const activeTab = tabs.getActive();
      if (activeTab) {
        editor.show(activeTab.fileId, activeTab.content);
      }
    }
    currentView = validationViewBeforeOpen;
    viewBtns.forEach((b) => b.classList.toggle("active", b.dataset.view === currentView));
  }

  isClosingValidation = false;
}

async function handleValidationValidate(fileId: string): Promise<void> {
  const activeTab = tabs.getActive();
  if (!activeTab || activeTab.fileId !== fileId) return;

  const modified = validationPanel.getModifiedContent();

  try {
    const result = await rpc<{ ok: boolean; content?: string }>("editor.validateChanges", {
      fileId,
      modified,
    });

    const newContent = result.content ?? modified;
    editor.setContent(newContent);
    tabs.updateContent(fileId, newContent);
    tabs.setDirty(fileId, false);
    toast.show("Modifications validées et sauvegardées.", "success", 2500);
    await closeValidationPanel(true);
  } catch (err) {
    toast.show("Erreur lors de la validation : " + String(err), "error");
  }
}

async function handleValidationCancel(fileId: string): Promise<void> {
  try {
    const result = await rpc<{ original: string }>("editor.discardChanges", { fileId });
    editor.setContent(result.original);
    tabs.updateContent(fileId, result.original);
    tabs.setDirty(fileId, false);
    toast.show("Modifications annulées.", "info", 2000);
    await closeValidationPanel(true);
  } catch (err) {
    toast.show("Erreur lors de l'annulation : " + String(err), "error");
  }
}

// ── Cursor tracking (debounce 100ms) ─────────────────────────────────────────
let cursorDebounce: ReturnType<typeof setTimeout> | null = null;
let sectionLocked = false;

function onCursorMove(fileId: string, line: number): void {
  if (sectionLocked) return;
  if (cursorDebounce !== null) clearTimeout(cursorDebounce);
  cursorDebounce = setTimeout(async () => {
    try {
      const ref = await rpc<SectionRef | null>("document.getActiveSection", {
        fileId,
        cursorLine: line,
      });
      updateSectionIndicator(ref);
      editor.setActiveSection(
        ref ? { startLine: ref.startLine, endLine: ref.endLine } : null
      );
    } catch {
      // silencieux
    }
  }, 100);
}

function updateSectionIndicator(ref: SectionRef | null): void {
  if (!ref) {
    sectionNameEl.textContent = "—";
    return;
  }
  const prefix = "#".repeat(ref.headingLevel);
  sectionNameEl.textContent = `${prefix} ${ref.headingTitle}`;
}

// ── Wrap toggle ───────────────────────────────────────────────────────────────
const WRAP_KEY = "amatled.lineWrapping";
let lineWrapping = localStorage.getItem(WRAP_KEY) !== "false";

function applyWrapState(enabled: boolean): void {
  lineWrapping = enabled;
  localStorage.setItem(WRAP_KEY, String(enabled));
  menuWrap.classList.toggle("active", enabled);
  editor.setLineWrapping(enabled);
}

menuWrap.addEventListener("click", () => applyWrapState(!lineWrapping));

// ── Thème de l'éditeur ───────────────────────────────────────────────────────
const THEME_KEY = "amatled.editorTheme";
const storedTheme = localStorage.getItem(THEME_KEY) as EditorThemeName | null;
let editorTheme: EditorThemeName =
  EDITOR_THEMES.some((t) => t.name === storedTheme) ? (storedTheme as EditorThemeName) : "dark";

function applyEditorTheme(theme: EditorThemeName): void {
  editorTheme = theme;
  localStorage.setItem(THEME_KEY, theme);
  menuThemeButtons.forEach((btn) => btn.classList.toggle("active", btn.dataset.theme === theme));
  editor.setTheme(theme);
}

menuThemeButtons.forEach((btn) => {
  btn.addEventListener("click", () => applyEditorTheme(btn.dataset.theme as EditorThemeName));
});

// ── Composants ────────────────────────────────────────────────────────────────
const editor = new Editor(
  editorContent,
  (fileId, dirty) => tabs.setDirty(fileId, dirty),
  (fileId, content) => tabs.updateContent(fileId, content),
  onCursorMove,
  lineWrapping,
  editorTheme
);

const chat = new Chat(chatMessagesEl, chatInputEl, chatSendBtn, chatCancelBtn);

const settingsModal = new SettingsModal(document.body);

const tabs = new TabManager(
  tabsBar,
  (tab) => {
    editorEmpty.classList.add("hidden");
    menuExportPdf.disabled = false;
    if (currentView === "preview") {
      editorContent.classList.add("hidden");
      previewContainer.classList.remove("hidden");
      preview.render(tab.fileId).catch(console.error);
    } else {
      editorContent.classList.remove("hidden");
      previewContainer.classList.add("hidden");
      editor.show(tab.fileId, tab.content);
    }
    tree.setActivePath(tab.path);
    updateSectionIndicator(null);
    chat.setActiveFile(tab.fileId);
  },
  () => {
    editor.hide();
    editorContent.classList.add("hidden");
    editorEmpty.classList.remove("hidden");
    menuExportPdf.disabled = true;
    chat.setActiveFile(null);
  }
);

// ── Ouverture d'un fichier dans l'éditeur ─────────────────────────────────────
async function openFileInEditor(filePath: string): Promise<void> {
  const existingTab = tabs.getTab(filePath);
  if (existingTab) {
    tabs.switchTo(filePath);
    tree.expandToPath(filePath);
    return;
  }

  try {
    const result = await rpc<OpenFileResult>("editor.openFile", { path: filePath });
    const fileName = filePath.split("/").pop() ?? filePath;
    tabs.open(filePath, fileName, result.content);
    tree.expandToPath(filePath);
  } catch (err) {
    console.error("editor.openFile failed", err);
    toast.show("Impossible d'ouvrir le fichier : " + String(err), "error");
  }
}

const tree = new FileTree(
  treeFiles,
  async (entry) => {
    await openFileInEditor(entry.path);
  },
  async (entry) => {
    if (!confirm(`Supprimer « ${entry.name} » ?`)) return;
    try {
      await rpc("workspace.deleteFile", { path: entry.path });
      tabs.close(entry.path);
    } catch (err) {
      toast.show("Suppression impossible : " + String(err), "error");
    }
  },
  async (dirPath) => {
    const name = prompt("Nom du fichier (ex: notes.md ou dossier/page.md) :");
    if (!name?.trim()) return;
    const trimmedName = name.trim();
    const fileName = trimmedName.endsWith('.md') ? trimmedName : `${trimmedName}.md`
    const filePath = dirPath ? `${dirPath}/${fileName}` : fileName;
    try {
      await rpc("workspace.createFile", { path: filePath });
      const result = await rpc<OpenFileResult>("editor.openFile", { path: filePath });
      const fileName = filePath.split("/").pop() ?? filePath;
      tabs.open(filePath, fileName, result.content);
    } catch (err) {
      toast.show("Création impossible : " + String(err), "error");
    }
  },
);

validationPanel = new ValidationPanel(validationPanelEl, {
  onValidate: handleValidationValidate,
  onCancel: handleValidationCancel,
});

// Sauvegarde normale => panel de validation. Sauvegarde forcée => save direct.
editor.setSaveCallbacks(openValidationPanel, forceSaveCurrentFile);

// ── Section lock ──────────────────────────────────────────────────────────────
async function toggleSectionLock(): Promise<void> {
  const activeTab = tabs.getActive();
  if (!activeTab) return;
  sectionLocked = !sectionLocked;
  sectionIndicator.title = sectionLocked
    ? "Section verrouillée (clic pour déverrouiller)"
    : "Clic pour verrouiller la section active";
  sectionIndicator.style.opacity = sectionLocked ? "1" : "0.7";
  sectionNameEl.style.color = sectionLocked
    ? "var(--success)"
    : "var(--warning)";
  try {
    await rpc("document.lockSection", {
      fileId: activeTab.fileId,
      locked: sectionLocked,
    });
    toast.show(sectionLocked ? "Section verrouillée." : "Section déverrouillée.", "info", 1800);
  } catch (err) {
    console.error("lockSection failed", err);
  }
}

sectionIndicator.addEventListener("click", toggleSectionLock);

// ── Ouverture d'un workspace ──────────────────────────────────────────────────
function applyWorkspace(result: WorkspaceResult): void {
  tree.setFiles(result.files);
  if (result.initialFile) {
    requestAnimationFrame(() => tree.expandToPath(result.initialFile!));
  }
  const parts = result.rootPath.split(/[\\/]/);
  const name = parts[parts.length - 1] || result.rootPath;
  document.title = `${name} — AmatlEd`;
}

async function openWorkspace(): Promise<void> {
  try {
    const sel = await rpc<{ path?: string; cancelled?: boolean }>(
      "workspace.selectFolder",
      {}
    );
    if (sel.cancelled || !sel.path) return;
    const result = await rpc<WorkspaceResult>("workspace.open", {
      path: sel.path,
    });
    applyWorkspace(result);
  } catch (err) {
    toast.show("Impossible d'ouvrir le workspace : " + String(err), "error");
    console.error("open workspace failed", err);
  }
}

menuOpenWorkspace.addEventListener("click", openWorkspace);

// ── Bouton Paramètres ─────────────────────────────────────────────────────────
btnSettings.addEventListener("click", () => settingsModal.open());

// ── Export PDF ────────────────────────────────────────────────────────────────
async function doExportPdf(): Promise<void> {
  const activeTab = tabs.getActive();
  if (!activeTab) return;
  const originalText = menuExportPdf.textContent!;
  menuExportPdf.disabled = true;
  menuExportPdf.textContent = "Export en cours…";
  try {
    const result = await rpc<{ ok: boolean; cancelled?: boolean }>("document.exportPDF", { fileId: activeTab.fileId });
    if (!result.cancelled) {
      toast.show("PDF exporté avec succès.", "success", 3000);
    }
  } catch (err) {
    toast.show("Erreur lors de l'export PDF : " + String(err), "error");
  } finally {
    menuExportPdf.disabled = !tabs.getActive();
    menuExportPdf.textContent = originalText;
  }
}

menuExportPdf.addEventListener("click", doExportPdf);

// ── Toggle chat panel ─────────────────────────────────────────────────────────
function toggleChatPanel(): void {
  const hidden = mainLayout.classList.toggle("chat-hidden");
  menuToggleChat.textContent = hidden ? "Afficher le panneau de chat" : "Masquer le panneau de chat";
}

menuToggleChat.addEventListener("click", toggleChatPanel);

// ── Nouvelle conversation (menu) ──────────────────────────────────────────────
menuNewChat.addEventListener("click", () => {
  chat.clearConversation();
  toast.show("Nouvelle conversation démarrée.", "info", 2000);
});

// ── Raccourcis clavier globaux ────────────────────────────────────────────────
document.addEventListener("keydown", (e) => {
  // F5 — basculer Source / Rendu
  if (e.key === "F5" && !e.ctrlKey && !e.altKey) {
    e.preventDefault();
    switchView(currentView === "source" ? "preview" : "source");
    return;
  }
  // Ctrl+O — ouvrir workspace
  if (e.key === "o" && e.ctrlKey && !e.shiftKey && !e.altKey) {
    if (document.activeElement?.tagName === "INPUT" || document.activeElement?.tagName === "TEXTAREA") return;
    e.preventDefault();
    openWorkspace();
    return;
  }
  // Ctrl+L — verrouiller / déverrouiller la section active
  if (e.key === "l" && e.ctrlKey && !e.shiftKey && !e.altKey) {
    e.preventDefault();
    toggleSectionLock();
    return;
  }
  // Ctrl+Shift+N — nouvelle conversation chat
  if (e.key === "N" && e.ctrlKey && e.shiftKey && !e.altKey) {
    e.preventDefault();
    chat.clearConversation();
    toast.show("Nouvelle conversation démarrée.", "info", 2000);
    return;
  }
  // Escape — fermer le panel de validation si ouvert ; sinon fermer menus/paramètres
  if (e.key === "Escape") {
    if (!validationPanelEl.classList.contains("hidden")) {
      const fileId = validationPanel.getFileId();
      if (fileId && confirmDiscardChanges()) {
        handleValidationCancel(fileId);
      }
      return;
    }
    closeAllMenus();
    settingsModal.close();
    return;
  }
});

// ── Événements Go → JS ───────────────────────────────────────────────────────
bus.on("workspace.opened", (data) => {
  const result = data as WorkspaceResult & { activeProfile?: string; activeRenderPreset?: string };
  applyWorkspace(result);
  if (result.initialFile && result.initialFileContent) {
    const fileName = result.initialFile.split("/").pop() ?? result.initialFile;
    tabs.open(result.initialFile, fileName, result.initialFileContent.content);
  }
  refreshProfileSelector(result.activeProfile);
  refreshRenderPresetSelector(result.activeRenderPreset ?? "");
});

bus.on("profile.changed", (data) => {
  const { name } = data as { name: string };
  if (profileSelectEl.value !== name) profileSelectEl.value = name;
});

bus.on("profiles.updated", () => {
  refreshProfileSelector();
});

bus.on("renderPreset.changed", (data) => {
  const { name } = data as { name: string };
  if (renderPresetSelectEl.value !== name) renderPresetSelectEl.value = name;
});

bus.on("renderPresets.updated", () => {
  refreshRenderPresetSelector();
});

bus.on("workspace.treeUpdated", (data) => {
  const { files } = data as { files: FileEntry[] };
  tree.setFiles(files);
});

bus.on("pdf-error", (data) => {
  const { fileId, error, path } = data as { fileId: string; error: string; path: string };
  const activeTab = tabs.getActive();
  if (activeTab?.fileId === fileId) {
    const msg = path ? `Ressource manquante : ${path}` : `Erreur de rendu : ${error}`;
    toast.show(msg, "error");
  }
});

// L'agent IA demande au frontend d'ouvrir le panel de validation.
bus.on("validation.show", async (data) => {
  const { fileId } = data as { fileId: string };
  const activeTab = tabs.getActive();
  if (!activeTab || activeTab.fileId !== fileId) {
    toast.show("Un panel de validation est disponible pour un autre fichier.", "info", 3000);
    return;
  }
  await openValidationPanel();
});

bus.on("editor.fileChangedOnDisk", (data) => {
  const { fileId } = data as { fileId: string };
  // N'afficher la bannière que si le fichier est actuellement actif
  const activeTab = tabs.getActive();
  if (activeTab?.fileId === fileId) {
    showFileChangedBanner(fileId);
  }
});

bus.on("updater.updateAvailable", (data) => {
  const { version } = data as { version: string };
  toast.show(
    `Mise à jour disponible : ${version}. Ouvrez les paramètres (⚙) pour l'installer.`,
    "info",
    10000
  );
});

bus.on("updater.updateApplied", (data) => {
  const { version } = data as { version: string };
  toast.show(
    `Mis à jour vers ${version}. Relancez l'application pour appliquer.`,
    "success",
    60000
  );
});

// ── Démarrage ─────────────────────────────────────────────────────────────────
// État initial : aucun fichier ouvert, éditeur masqué
editorContent.classList.add("hidden");
// Synchroniser l'état du wrap et du thème dans le menu
applyWrapState(lineWrapping);
menuThemeButtons.forEach((btn) => btn.classList.toggle("active", btn.dataset.theme === editorTheme));

(async () => {
  // Attend que lorca ait bindé window.rpc avant tout appel.
  await waitForBridge();
  try {
    const s = await rpc<Settings>("settings.get", {});
    if (!s) {
      toast.show("Configurez votre provider LLM dans les paramètres (⚙).", "warning", 6000);
    }
  } catch (err) {
    console.error("settings.get failed", err);
  }
})();
