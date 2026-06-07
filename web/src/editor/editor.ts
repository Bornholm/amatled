import { Annotation, Compartment } from "@codemirror/state";
import { EditorView, keymap, lineNumbers, highlightActiveLine,
         highlightActiveLineGutter } from "@codemirror/view";
import { EditorState } from "@codemirror/state";
import { markdown } from "@codemirror/lang-markdown";
import { defaultKeymap, indentWithTab } from "@codemirror/commands";
import { bus } from "../bridge";
import { rpc } from "../bridge";
import { darkTheme, markdownHighlight } from "./theme";
import { sectionGutterExtension, setSectionEffect, SectionRange } from "./section-gutter";

// Annotation pour marquer les transactions venant de Go (undo/redo/agent)
// afin d'éviter les boucles de synchronisation.
const remoteAnnotation = Annotation.define<boolean>();

interface Change {
  from: { line: number; column: number };
  to: { line: number; column: number };
  insert: string;
}

interface UndoRedoResult {
  content?: string;
  noop?: boolean;
}

interface ContentUpdatedEvent {
  fileId: string;
  content: string;
  source: "human" | "agent" | string;
}

function endPosition(content: string): { line: number; column: number } {
  const lines = content.split("\n");
  const lastLine = lines.length - 1;
  return { line: lastLine, column: [...lines[lastLine]].length };
}

function makeWholeDocChange(oldContent: string, newContent: string): Change {
  return {
    from: { line: 0, column: 0 },
    to: endPosition(oldContent),
    insert: newContent,
  };
}

export class Editor {
  private container: HTMLElement;
  private view: EditorView | null = null;
  private currentFileId: string | null = null;
  private pendingOldContent: string | null = null;
  private debounceTimer: ReturnType<typeof setTimeout> | null = null;
  private cursorTimer: ReturnType<typeof setTimeout> | null = null;
  private onDirty: (fileId: string, dirty: boolean) => void;
  private onContentChange: (fileId: string, content: string) => void;
  private onCursorMove: (fileId: string, line: number) => void;
  private unsubContentUpdated: (() => void) | null = null;
  private lineWrapping = true;
  private wrapCompartment = new Compartment();

  constructor(
    container: HTMLElement,
    onDirty: (fileId: string, dirty: boolean) => void,
    onContentChange: (fileId: string, content: string) => void,
    onCursorMove: (fileId: string, line: number) => void,
    lineWrapping = true
  ) {
    this.lineWrapping = lineWrapping;
    this.container = container;
    this.onDirty = onDirty;
    this.onContentChange = onContentChange;
    this.onCursorMove = onCursorMove;

    // Écouter les mises à jour poussées par Go (undo/redo/agent)
    this.unsubContentUpdated = bus.on(
      "editor.contentUpdated",
      (data: unknown) => {
        const ev = data as ContentUpdatedEvent;
        if (ev.fileId !== this.currentFileId) return;
        // Les changements "human" viennent de l'éditeur lui-même — les réappliquer
        // remplacerait le document et repositionnerait le curseur au début.
        if (ev.source === "human") return;
        this.applyRemoteContent(ev.content);
        this.onDirty(ev.fileId, true);
      }
    );

    this.renderEmpty();
  }

  show(fileId: string, content: string): void {
    // Détruire la vue précédente si elle existe
    if (this.view) {
      this.view.destroy();
      this.view = null;
    }
    this.clearTimers();
    this.container.innerHTML = "";
    this.currentFileId = fileId;
    this.pendingOldContent = null;

    const self = this;

    const state = EditorState.create({
      doc: content,
      extensions: [
        lineNumbers(),
        highlightActiveLine(),
        highlightActiveLineGutter(),
        markdown(),
        darkTheme,
        markdownHighlight,
        sectionGutterExtension,
        // Keymap personnalisé — PAS de history CM6 (géré par Go)
        keymap.of([
          {
            key: "Ctrl-z",
            mac: "Cmd-z",
            run: () => { self.undo(); return true; },
            preventDefault: true,
          },
          {
            key: "Ctrl-y",
            mac: "Cmd-Shift-z",
            run: () => { self.redo(); return true; },
            preventDefault: true,
          },
          {
            key: "Ctrl-Shift-z",
            run: () => { self.redo(); return true; },
            preventDefault: true,
          },
          {
            key: "Ctrl-s",
            mac: "Cmd-s",
            run: () => { self.save(); return true; },
            preventDefault: true,
          },
          indentWithTab,
          ...defaultKeymap,
        ]),
        // Listener bidirectionnel CM6 ↔ Go
        EditorView.updateListener.of((update) => {
          const isRemote = update.transactions.some(
            (t) => t.annotation(remoteAnnotation)
          );
          if (update.docChanged && !isRemote) {
            self.scheduleSync(update.startState.doc.toString());
          }
          if ((update.selectionSet || update.docChanged) && !isRemote) {
            self.scheduleCursorUpdate();
          }
        }),
        this.wrapCompartment.of(this.lineWrapping ? EditorView.lineWrapping : []),
        EditorView.theme({
          "&": { flex: "1", minHeight: "0" },
        }),
      ],
    });

    this.view = new EditorView({ state, parent: this.container });
    this.view.focus();
  }

  // Mettre à jour la section active surlignée dans l'éditeur.
  setActiveSection(range: SectionRange | null): void {
    if (!this.view) return;
    this.view.dispatch({
      effects: setSectionEffect.of(range),
      annotations: remoteAnnotation.of(true),
    });
  }

  setLineWrapping(enabled: boolean): void {
    this.lineWrapping = enabled;
    if (!this.view) return;
    this.view.dispatch({
      effects: this.wrapCompartment.reconfigure(enabled ? EditorView.lineWrapping : []),
    });
  }

  setContent(content: string): void {
    this.applyRemoteContent(content);
  }

  getContent(): string {
    return this.view?.state.doc.toString() ?? "";
  }

  hide(): void {
    this.clearTimers();
    this.view?.destroy();
    this.view = null;
    this.currentFileId = null;
    this.pendingOldContent = null;
    this.renderEmpty();
  }

  destroy(): void {
    this.clearTimers();
    this.view?.destroy();
    this.unsubContentUpdated?.();
  }

  // ── Privé ─────────────────────────────────────────────────────────────────

  private scheduleSync(currentOldContent: string): void {
    // Capture l'état avant la séquence de frappes
    if (this.pendingOldContent === null) {
      this.pendingOldContent = currentOldContent;
    }
    if (this.debounceTimer !== null) clearTimeout(this.debounceTimer);

    const fileId = this.currentFileId!;
    this.debounceTimer = setTimeout(async () => {
      this.debounceTimer = null;
      const oldContent = this.pendingOldContent!;
      const newContent = this.view?.state.doc.toString() ?? "";
      this.pendingOldContent = null;

      if (oldContent === newContent) return;

      this.onDirty(fileId, true);
      this.onContentChange(fileId, newContent);
      try {
        await rpc("editor.applyLocalChanges", {
          fileId,
          changes: [makeWholeDocChange(oldContent, newContent)],
        });
      } catch (err) {
        console.error("applyLocalChanges failed", err);
      }
    }, 200);
  }

  private scheduleCursorUpdate(): void {
    if (this.cursorTimer !== null) clearTimeout(this.cursorTimer);
    this.cursorTimer = setTimeout(() => {
      this.cursorTimer = null;
      if (!this.view || !this.currentFileId) return;
      const pos = this.view.state.selection.main.head;
      const line = this.view.state.doc.lineAt(pos);
      this.onCursorMove(this.currentFileId, line.number - 1); // 0-indexed
    }, 100);
  }

  private applyRemoteContent(newContent: string): void {
    if (!this.view) return;
    const curLen = this.view.state.doc.length;
    const scrollTop = this.view.scrollDOM.scrollTop;
    const cursorPos = this.view.state.selection.main.head;

    this.view.dispatch({
      changes: { from: 0, to: curLen, insert: newContent },
      annotations: remoteAnnotation.of(true),
    });

    const newLen = this.view.state.doc.length;
    const restorePos = Math.min(cursorPos, newLen);
    this.view.dispatch({
      selection: { anchor: restorePos },
      annotations: remoteAnnotation.of(true),
    });

    if (scrollTop > 0) {
      requestAnimationFrame(() => {
        if (this.view) {
          this.view.scrollDOM.scrollTop = Math.min(scrollTop, this.view.scrollDOM.scrollHeight);
        }
      });
    }

    this.pendingOldContent = null;
  }

  private async undo(): Promise<void> {
    if (!this.currentFileId) return;
    // Flush du debounce avant undo pour éviter des conflits
    await this.flushSync();
    try {
      const result = await rpc<UndoRedoResult>("history.undo", {
        fileId: this.currentFileId,
      });
      if (!result.noop && result.content !== undefined) {
        this.applyRemoteContent(result.content);
        this.onDirty(this.currentFileId, true);
      }
    } catch (err) {
      console.error("undo failed", err);
    }
  }

  private async redo(): Promise<void> {
    if (!this.currentFileId) return;
    try {
      const result = await rpc<UndoRedoResult>("history.redo", {
        fileId: this.currentFileId,
      });
      if (!result.noop && result.content !== undefined) {
        this.applyRemoteContent(result.content);
        this.onDirty(this.currentFileId, true);
      }
    } catch (err) {
      console.error("redo failed", err);
    }
  }

  private async save(): Promise<void> {
    if (!this.currentFileId) return;
    await this.flushSync();
    try {
      const result = await rpc<{ ok: boolean; content?: string }>("editor.saveFile", { fileId: this.currentFileId });
      if (result.content !== undefined) {
        this.applyRemoteContent(result.content);
        this.onContentChange(this.currentFileId, result.content);
      }
      this.onDirty(this.currentFileId, false);
    } catch (err) {
      console.error("save failed", err);
    }
  }

  private async flushSync(): Promise<void> {
    if (this.debounceTimer !== null) {
      clearTimeout(this.debounceTimer);
      this.debounceTimer = null;
      if (!this.view || !this.currentFileId || this.pendingOldContent === null) return;
      const oldContent = this.pendingOldContent;
      const newContent = this.view.state.doc.toString();
      this.pendingOldContent = null;
      if (oldContent !== newContent) {
        await rpc("editor.applyLocalChanges", {
          fileId: this.currentFileId,
          changes: [makeWholeDocChange(oldContent, newContent)],
        }).catch(() => {});
      }
    }
  }

  private clearTimers(): void {
    if (this.debounceTimer !== null) { clearTimeout(this.debounceTimer); this.debounceTimer = null; }
    if (this.cursorTimer !== null) { clearTimeout(this.cursorTimer); this.cursorTimer = null; }
  }

  private renderEmpty(): void {
    this.container.innerHTML =
      '<div id="editor-empty">Ouvrez un fichier pour commencer</div>';
  }
}
