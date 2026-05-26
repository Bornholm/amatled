import { rpc, bus, waitForBridge } from "./bridge";
import { FileTree, FileEntry } from "./tree/tree";
import { TabManager } from "./tabs/tabs";
import { Editor } from "./editor/editor";
import { Preview } from "./preview/preview";
import { Chat } from "./chat/chat";
import { SettingsModal } from "./settings/settings";
import { toast } from "./toast/toast";

interface Settings {
  lastWorkspace: string;
}

interface WorkspaceResult {
  files: FileEntry[];
  rootPath: string;
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

// ── Éléments DOM ──────────────────────────────────────────────────────────────
const btnOpenWorkspace = document.getElementById("btn-open-workspace")!;
const btnSettings = document.getElementById("btn-settings")!;
const workspaceLabel = document.getElementById("workspace-label")!;
const treePanel = document.getElementById("tree-panel")!;
const tabsBar = document.getElementById("tabs-bar")!;
const editorContent = document.getElementById("editor-content")!;
const previewContainer = document.getElementById("preview-container")!;
const previewFrame = document.getElementById("preview-frame") as HTMLIFrameElement;
const previewSpinner = previewContainer.querySelector<HTMLElement>(".preview-spinner")!;
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

// ── Preview & toggle Source/Mise en forme ──────────────────────────────────────
const preview = new Preview(previewFrame, previewSpinner);
let currentView: "source" | "preview" = "source";

async function switchView(view: "source" | "preview"): Promise<void> {
  if (view === currentView) return;
  currentView = view;
  viewBtns.forEach((b) => b.classList.toggle("active", b.dataset.view === view));
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
const btnWrapToggle = document.getElementById("btn-wrap-toggle") as HTMLButtonElement;
let lineWrapping = localStorage.getItem(WRAP_KEY) !== "false";

function applyWrapState(enabled: boolean): void {
  lineWrapping = enabled;
  localStorage.setItem(WRAP_KEY, String(enabled));
  btnWrapToggle.classList.toggle("btn--active", enabled);
  btnWrapToggle.title = enabled
    ? "Retour à la ligne automatique (actif)"
    : "Retour à la ligne automatique (inactif)";
  editor.setLineWrapping(enabled);
}

btnWrapToggle.addEventListener("click", () => applyWrapState(!lineWrapping));

// ── Composants ────────────────────────────────────────────────────────────────
const editor = new Editor(
  editorContent,
  (fileId, dirty) => tabs.setDirty(fileId, dirty),
  (fileId, content) => tabs.updateContent(fileId, content),
  onCursorMove,
  lineWrapping
);

const chat = new Chat(chatMessagesEl, chatInputEl, chatSendBtn, chatCancelBtn);

const settingsModal = new SettingsModal(document.body);

const tabs = new TabManager(
  tabsBar,
  (tab) => {
    if (currentView === "preview") {
      preview.render(tab.fileId).catch(console.error);
    } else {
      editor.show(tab.fileId, tab.content);
    }
    tree.setActivePath(tab.path);
    updateSectionIndicator(null);
    chat.setActiveFile(tab.fileId);
  },
  () => {
    editor.hide();
    chat.setActiveFile(null);
  }
);

const tree = new FileTree(treePanel, async (entry) => {
  const existingTab = tabs.getTab(entry.path);
  if (existingTab) {
    tabs.switchTo(entry.path);
    return;
  }
  try {
    const result = await rpc<OpenFileResult>("editor.openFile", {
      path: entry.path,
    });
    tabs.open(entry.path, entry.name, result.content);
  } catch (err) {
    console.error("editor.openFile failed", err);
  }
});

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
  const parts = result.rootPath.split(/[\\/]/);
  workspaceLabel.textContent = parts[parts.length - 1] || result.rootPath;
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

btnOpenWorkspace.addEventListener("click", openWorkspace);

// ── Bouton Paramètres ─────────────────────────────────────────────────────────
btnSettings.addEventListener("click", () => {
  settingsModal.open();
});

// ── Raccourcis clavier globaux ────────────────────────────────────────────────
document.addEventListener("keydown", (e) => {
  // F5 — basculer Source / Mise en forme
  if (e.key === "F5" && !e.ctrlKey && !e.altKey) {
    e.preventDefault();
    switchView(currentView === "source" ? "preview" : "source");
    return;
  }
  // Ctrl+O — ouvrir workspace
  if (e.key === "o" && e.ctrlKey && !e.shiftKey && !e.altKey) {
    // Ne pas interférer si le focus est dans un champ
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
  // Escape — fermer la modale paramètres
  if (e.key === "Escape") {
    settingsModal.close();
    return;
  }
});

// ── Événements Go → JS ───────────────────────────────────────────────────────
bus.on("workspace.opened", (data) => {
  applyWorkspace(data as WorkspaceResult);
});

bus.on("workspace.treeUpdated", (data) => {
  const { files } = data as { files: FileEntry[] };
  tree.setFiles(files);
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
