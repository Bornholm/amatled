import { EditorView } from "@codemirror/view";

export function getSelection(view: EditorView): { from: number; to: number; text: string } {
  const selection = view.state.selection.main;
  return {
    from: selection.from,
    to: selection.to,
    text: view.state.sliceDoc(selection.from, selection.to),
  };
}

export function wrapSelection(
  view: EditorView,
  before: string,
  after: string,
  placeholder: string = ""
): void {
  const { from, to, text } = getSelection(view);
  const insert = text.length > 0 ? before + text + after : before + placeholder + after;
  const cursorOffset = text.length > 0 ? before.length : before.length + placeholder.length;

  view.dispatch({
    changes: { from, to, insert },
    selection: { anchor: from + cursorOffset, head: from + cursorOffset + (text.length > 0 ? text.length : placeholder.length) },
  });
  view.focus();
}

export function insertAtLineStart(view: EditorView, prefix: string): void {
  const selection = view.state.selection.main;
  const line = view.state.doc.lineAt(selection.from);

  view.dispatch({
    changes: { from: line.from, to: line.from, insert: prefix },
  });
  view.focus();
}

export function insertBlock(view: EditorView, block: string): void {
  const { from, to } = getSelection(view);
  const line = view.state.doc.lineAt(from);

  view.dispatch({
    changes: { from, to, insert: block },
    selection: { anchor: from + block.length },
  });
  view.focus();
}

export function insertMultilineBlock(view: EditorView, block: string): void {
  const selection = view.state.selection.main;
  const line = view.state.doc.lineAt(selection.from);
  const insert = "\n" + block + "\n";

  view.dispatch({
    changes: { from: line.from, to: line.to, insert },
    selection: { anchor: line.from + insert.length },
  });
  view.focus();
}

export function generateTable(rows: number, cols: number): string {
  const header = "| " + Array.from({ length: cols }, (_, i) => `Colonne ${i + 1}`).join(" | ") + " |";
  const separator = "| " + Array.from({ length: cols }, () => "---").join(" | ") + " |";
  const bodyRows = Array.from({ length: rows - 1 }, () =>
    "| " + Array.from({ length: cols }, () => "").join(" | ") + " |"
  ).join("\n");

  return [header, separator, bodyRows].join("\n");
}

export function insertLink(view: EditorView): void {
  const { text } = getSelection(view);
  const url = prompt("URL du lien :", text.startsWith("http") ? text : "https://");
  if (url) {
    const label = text.length > 0 && !text.startsWith("http") ? text : "lien";
    wrapSelection(view, "[", `](${url})`, label);
  }
}

export function insertImage(view: EditorView): void {
  const url = prompt("URL de l'image :", "https://");
  if (url) {
    const alt = prompt("Texte alternatif :", "image");
    wrapSelection(view, "![", `](${url})`, alt || "image");
  }
}

export function insertTableCmd(view: EditorView): void {
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

export function insertHr(view: EditorView): void {
  const selection = view.state.selection.main;
  const line = view.state.doc.lineAt(selection.from);
  const insert = line.from > 0 && view.state.sliceDoc(line.from - 1, line.from) !== "\n" ? "\n---\n" : "---\n";
  view.dispatch({
    changes: { from: line.from, to: line.from, insert },
  });
  view.focus();
}
