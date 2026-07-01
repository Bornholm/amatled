import { EditorView } from "@codemirror/view";
import { wrapSelection, insertAtLineStart, insertMultilineBlock, generateTable } from "./formatting";

interface ToolbarButton {
  id: string;
  title: string;
  shortcut?: string;
  svg: string;
  action: (view: EditorView) => void;
}

const ICONS = {
  bold: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M6 4h8a4 4 0 0 1 4 4 4 4 0 0 1-4 4H6z"/><path d="M6 12h9a4 4 0 0 1 4 4 4 4 0 0 1-4 4H6z"/></svg>`,
  italic: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="19" y1="4" x2="10" y2="4"/><line x1="14" y1="20" x2="5" y2="20"/><line x1="15" y1="4" x2="9" y2="20"/></svg>`,
  strikethrough: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17.3 4.9c-2.3-.6-4.4-1-6.2-.9-2.7 0-5.3.7-5.3 3.6 0 1.5 1.8 3.3 6 3.9h.5m5.7 3.1c.4.4.6.9.6 1.3 0 2.9-3.7 4.2-7.6 4.2-3.2 0-6.3-.8-8.4-2.3"/></svg>`,
  h1: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 12h8"/><path d="M4 18V6"/><path d="M12 18V6"/><path d="m17 12 3-2v8"/></svg>`,
  h2: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 12h8"/><path d="M4 18V6"/><path d="M12 18V6"/><path d="M21 18h-4c0-4 4-3 4-6 0-1.5-2-2.5-4-1"/></svg>`,
  h3: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 12h8"/><path d="M4 18V6"/><path d="M12 18V6"/><path d="M17.5 10.5c1.7-1 3.5 0 3.5 1.5a2 2 0 0 1-2 2"/><path d="M17 17.5c2 1.5 4 .3 4-1.5a2 2 0 0 0-2-2"/></svg>`,
  quote: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 21c3 0 7-1 7-8V5c0-1.25-.756-2.017-2-2H4c-1.25 0-2 .75-2 1.972V11c0 1.25.75 2 2 2 1 0 1 0 1 1v1c0 1-1 2-2 2s-1 .008-1 1.031V21z"/><path d="M15 21c3 0 7-1 7-8V5c0-1.25-.757-2.017-2-2h-4c-1.25 0-2 .75-2 1.972V11c0 1.25.75 2 2 2h.75c0 2.25.25 4-2.75 4v3z"/></svg>`,
  code: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/></svg>`,
  codeBlock: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="2"/><path d="M8 10l4 4 4-4"/><line x1="4" y1="14" x2="20" y2="14"/></svg>`,
  link: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/></svg>`,
  image: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="9" cy="9" r="2"/><path d="m21 15-3.086-3.086a2 2 0 0 0-2.828 0L6 21"/></svg>`,
  ul: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="9" y1="6" x2="20" y2="6"/><line x1="9" y1="12" x2="20" y2="12"/><line x1="9" y1="18" x2="20" y2="18"/><circle cx="4" cy="6" r="1" fill="currentColor" stroke="none"/><circle cx="4" cy="12" r="1" fill="currentColor" stroke="none"/><circle cx="4" cy="18" r="1" fill="currentColor" stroke="none"/></svg>`,
  ol: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="10" y1="6" x2="21" y2="6"/><line x1="10" y1="12" x2="21" y2="12"/><line x1="10" y1="18" x2="21" y2="18"/><path d="M4 6h1v4"/><path d="M4 10h2"/><path d="M6 18H4c0-1 2-2 2-3s-1-1.5-2-1"/></svg>`,
  task: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="5" width="6" height="6" rx="1"/><path d="m3 17 2 2 4-4"/><line x1="13" y1="6" x2="21" y2="6"/><line x1="13" y1="12" x2="21" y2="12"/><line x1="13" y1="18" x2="21" y2="18"/></svg>`,
  table: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="2"/><line x1="3" y1="9" x2="21" y2="9"/><line x1="3" y1="15" x2="21" y2="15"/><line x1="9" y1="3" x2="9" y2="21"/><line x1="15" y1="3" x2="15" y2="21"/></svg>`,
  hr: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="3" y1="12" x2="21" y2="12"/><line x1="3" y1="6" x2="21" y2="6"/><line x1="3" y1="18" x2="21" y2="18"/></svg>`,
};

export class Toolbar {
  private container: HTMLElement;
  private view: EditorView | null = null;
  private buttons: Map<string, HTMLButtonElement> = new Map();

  constructor(container: HTMLElement) {
    this.container = container;
    this.render();
  }

  setEditorView(view: EditorView): void {
    this.view = view;
    view.state.selection;
    view.dom.addEventListener("keydown", () => this.updateButtonStates());
  }

  private createButton(config: ToolbarButton): HTMLButtonElement {
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "toolbar-btn";
    btn.id = `toolbar-${config.id}`;
    btn.title = config.shortcut ? `${config.title} (${config.shortcut})` : config.title;
    btn.innerHTML = config.svg;
    btn.setAttribute("aria-label", config.title);
    btn.setAttribute("data-toolbar-action", config.id);
    btn.addEventListener("click", () => {
      if (this.view) {
        config.action(this.view);
      }
    });
    return btn;
  }

  private createSep(): HTMLElement {
    const sep = document.createElement("div");
    sep.className = "toolbar-sep";
    return sep;
  }

  private wrap(action: (v: EditorView) => void) {
    return (view: EditorView) => {
      action(view);
      this.updateButtonStates();
    };
  }

  private render(): void {
    const buttons: Array<ToolbarButton | "sep"> = [
      { id: "bold", title: "Gras", shortcut: "Ctrl+B", svg: ICONS.bold, action: this.wrap((v) => wrapSelection(v, "**", "**", "texte")) },
      { id: "italic", title: "Italique", shortcut: "Ctrl+I", svg: ICONS.italic, action: this.wrap((v) => wrapSelection(v, "*", "*", "texte")) },
      { id: "strikethrough", title: "Barré", svg: ICONS.strikethrough, action: this.wrap((v) => wrapSelection(v, "~~", "~~", "texte")) },
      "sep",
      { id: "h1", title: "Titre 1", shortcut: "Ctrl+1", svg: ICONS.h1, action: this.wrap((v) => insertAtLineStart(v, "# ")) },
      { id: "h2", title: "Titre 2", shortcut: "Ctrl+2", svg: ICONS.h2, action: this.wrap((v) => insertAtLineStart(v, "## ")) },
      { id: "h3", title: "Titre 3", shortcut: "Ctrl+3", svg: ICONS.h3, action: this.wrap((v) => insertAtLineStart(v, "### ")) },
      "sep",
      { id: "quote", title: "Citation", svg: ICONS.quote, action: this.wrap((v) => insertAtLineStart(v, "> ")) },
      { id: "code", title: "Code inline", shortcut: "Ctrl+E", svg: ICONS.code, action: this.wrap((v) => wrapSelection(v, "`", "`", "code")) },
      { id: "code-block", title: "Bloc de code", shortcut: "Ctrl+Shift+C", svg: ICONS.codeBlock, action: this.wrap((v) => insertMultilineBlock(v, "```\n\n```")) },
      "sep",
      { id: "link", title: "Lien", shortcut: "Ctrl+K", svg: ICONS.link, action: this.wrap((v) => this.promptLink(v)) },
      { id: "image", title: "Image", svg: ICONS.image, action: this.wrap((v) => this.promptImage(v)) },
      "sep",
      { id: "ul", title: "Liste à puces", svg: ICONS.ul, action: this.wrap((v) => insertAtLineStart(v, "- ")) },
      { id: "ol", title: "Liste numérotée", svg: ICONS.ol, action: this.wrap((v) => insertAtLineStart(v, "1. ")) },
      { id: "task", title: "Liste de tâches", svg: ICONS.task, action: this.wrap((v) => insertAtLineStart(v, "- [ ] ")) },
      "sep",
      { id: "table", title: "Tableau", svg: ICONS.table, action: this.wrap((v) => this.insertTable(v)) },
      { id: "hr", title: "Séparateur horizontal", svg: ICONS.hr, action: this.wrap((v) => this.insertHr(v)) },
    ];

    this.container.innerHTML = "";
    let sepCount = 0;

    for (const item of buttons) {
      if (item === "sep") {
        sepCount++;
        this.container.appendChild(this.createSep());
      } else {
        const btn = this.createButton(item);
        this.buttons.set(item.id, btn);
        this.container.appendChild(btn);
      }
    }

    this.updateButtonStates();
  }

  private promptLink(view: EditorView): void {
    const { text } = (({ from, to, text }) => ({ from, to, text }))(view.state.selection.main);
    const url = prompt("URL du lien :", text.startsWith("http") ? text : "https://");
    if (url) {
      const label = text.length > 0 && !text.startsWith("http") ? text : "lien";
      wrapSelection(view, "[", `](${url})`, label);
    }
  }

  private promptImage(view: EditorView): void {
    const url = prompt("URL de l'image :", "https://");
    if (url) {
      const alt = prompt("Texte alternatif :", "image");
      wrapSelection(view, "![", `](${url})`, alt || "image");
    }
  }

  private insertTable(view: EditorView): void {
    const rows = parseInt(prompt("Nombre de lignes :", "3") || "3", 10);
    const cols = parseInt(prompt("Nombre de colonnes :", "3") || "3", 10);
    if (rows > 0 && cols > 0) {
      const table = generateTable(rows, cols);
      const selection = view.state.selection.main;
      const line = view.state.doc.lineAt(selection.from);
      view.dispatch({
        changes: { from: line.from, to: line.from, insert: table + "\n" },
      });
    }
  }

  private insertHr(view: EditorView): void {
    const selection = view.state.selection.main;
    const line = view.state.doc.lineAt(selection.from);
    const insert = line.from > 0 && view.state.sliceDoc(line.from - 1, line.from) !== "\n" ? "\n---\n" : "---\n";
    view.dispatch({
      changes: { from: line.from, to: line.from, insert },
    });
  }

  updateButtonStates(): void {
    // Enable all buttons when editor has focus
    // Could be enhanced to disable contextually (e.g., code block button inside code)
  }
}
