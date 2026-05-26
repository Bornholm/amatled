import { EditorView } from "@codemirror/view";
import { syntaxHighlighting, defaultHighlightStyle } from "@codemirror/language";

export const darkTheme = EditorView.theme(
  {
    "&": {
      height: "100%",
      backgroundColor: "var(--bg)",
      color: "var(--fg)",
    },
    ".cm-scroller": {
      fontFamily: "var(--font-mono)",
      fontSize: "14px",
      lineHeight: "1.6",
      overflow: "auto",
    },
    ".cm-content": {
      caretColor: "var(--fg)",
      padding: "16px 0",
    },
    ".cm-line": {
      padding: "0 20px",
    },
    "&.cm-focused": {
      outline: "none",
    },
    "&.cm-focused .cm-cursor": {
      borderLeftColor: "var(--fg)",
    },
    "&.cm-focused .cm-selectionBackground, ::selection": {
      backgroundColor: "#264f78",
    },
    ".cm-gutters": {
      backgroundColor: "var(--bg-secondary)",
      color: "var(--fg-muted)",
      borderRight: "1px solid var(--border)",
      minWidth: "32px",
    },
    ".cm-lineNumbers .cm-gutterElement": {
      padding: "0 8px 0 4px",
      minWidth: "28px",
    },
    ".cm-activeLine": {
      backgroundColor: "#2a2d2e",
    },
    ".cm-activeLineGutter": {
      backgroundColor: "#2a2d2e",
    },
    ".cm-selectionMatch": {
      backgroundColor: "#3a4a5e",
    },
    ".cm-matchingBracket": {
      backgroundColor: "#3b514d",
      outline: "1px solid #4ec9b0",
    },
    // Coloration syntaxique Markdown
    ".cm-header": { color: "#569cd6", fontWeight: "bold" },
    ".cm-strong": { fontWeight: "bold" },
    ".cm-em": { fontStyle: "italic" },
    ".cm-link": { color: "#4ec9b0", textDecoration: "underline" },
    ".cm-url": { color: "#4ec9b0" },
    ".cm-quote": { color: "#6a9955" },
    ".cm-code": { color: "#ce9178", fontFamily: "var(--font-mono)" },
  },
  { dark: true }
);

export const markdownHighlight = syntaxHighlighting(defaultHighlightStyle);
