import { EditorView } from "@codemirror/view";
import { HighlightStyle, syntaxHighlighting } from "@codemirror/language";
import { tags } from "@lezer/highlight";
import type { Extension } from "@codemirror/state";

export type EditorThemeName = "dark" | "dark-contrast" | "light";

export const EDITOR_THEMES: { name: EditorThemeName; label: string }[] = [
  { name: "dark", label: "Sombre" },
  { name: "dark-contrast", label: "Sombre contrasté" },
  { name: "light", label: "Clair" },
];

interface Palette {
  dark: boolean;
  bg: string;
  bgGutter: string;
  fg: string;
  fgMuted: string;
  border: string;
  caret: string;
  selection: string;
  activeLine: string;
  selectionMatch: string;
  matchBg: string;
  matchOutline: string;
  heading: string;
  strong: string;
  link: string;
  url: string;
  quote: string;
  code: string;
  codeBg: string;
  markup: string;
  comment: string;
}

const palettes: Record<EditorThemeName, Palette> = {
  dark: {
    dark: true,
    bg: "#1e1e1e",
    bgGutter: "#252526",
    fg: "#d4d4d4",
    fgMuted: "#858585",
    border: "#3e3e42",
    caret: "#d4d4d4",
    selection: "#264f78",
    activeLine: "#2a2d2e",
    selectionMatch: "#3a4a5e",
    matchBg: "#3b514d",
    matchOutline: "#4ec9b0",
    heading: "#4fc1ff",
    strong: "#ffffff",
    link: "#4ec9b0",
    url: "#56b6c2",
    quote: "#8fbf5f",
    code: "#ce9178",
    codeBg: "#2d2d30",
    markup: "#6e7681",
    comment: "#7a9d6f",
  },
  "dark-contrast": {
    dark: true,
    bg: "#000000",
    bgGutter: "#101010",
    fg: "#ffffff",
    fgMuted: "#b0b0b0",
    border: "#444444",
    caret: "#ffffff",
    selection: "#005f9e",
    activeLine: "#1c1c1c",
    selectionMatch: "#3a4a5e",
    matchBg: "#1f5c52",
    matchOutline: "#6cf2d6",
    heading: "#7fd6ff",
    strong: "#ffffff",
    link: "#79f2c0",
    url: "#79ddee",
    quote: "#b5e88a",
    code: "#ffb38a",
    codeBg: "#1c1c1c",
    markup: "#9e9e9e",
    comment: "#9fcf8f",
  },
  light: {
    dark: false,
    bg: "#ffffff",
    bgGutter: "#f3f3f3",
    fg: "#1f1f1f",
    fgMuted: "#6e6e6e",
    border: "#d6d6d6",
    caret: "#1f1f1f",
    selection: "#add6ff",
    activeLine: "#f0f0f0",
    selectionMatch: "#d8e8ff",
    matchBg: "#bbeedd",
    matchOutline: "#2aa198",
    heading: "#0b5fa5",
    strong: "#000000",
    link: "#0e7a6a",
    url: "#0e7a8c",
    quote: "#4a7a1f",
    code: "#a13e1f",
    codeBg: "#f0f0f0",
    markup: "#8a8a8a",
    comment: "#3f7a30",
  },
};

function buildChromeTheme(p: Palette): Extension {
  return EditorView.theme(
    {
      "&": {
        height: "100%",
        backgroundColor: p.bg,
        color: p.fg,
      },
      ".cm-scroller": {
        fontFamily: "var(--font-mono)",
        fontSize: "14px",
        lineHeight: "1.6",
        overflow: "auto",
      },
      ".cm-content": {
        caretColor: p.caret,
        padding: "16px 0",
      },
      ".cm-line": {
        padding: "0 20px",
      },
      "&.cm-focused": {
        outline: "none",
      },
      "&.cm-focused .cm-cursor": {
        borderLeftColor: p.caret,
      },
      "&.cm-focused .cm-selectionBackground, ::selection": {
        backgroundColor: p.selection,
      },
      ".cm-gutters": {
        backgroundColor: p.bgGutter,
        color: p.fgMuted,
        borderRight: `1px solid ${p.border}`,
        minWidth: "32px",
      },
      ".cm-lineNumbers .cm-gutterElement": {
        padding: "0 8px 0 4px",
        minWidth: "28px",
      },
      ".cm-activeLine": {
        backgroundColor: p.activeLine,
      },
      ".cm-activeLineGutter": {
        backgroundColor: p.activeLine,
      },
      ".cm-selectionMatch": {
        backgroundColor: p.selectionMatch,
      },
      ".cm-matchingBracket": {
        backgroundColor: p.matchBg,
        outline: `1px solid ${p.matchOutline}`,
      },
    },
    { dark: p.dark }
  );
}

function buildHighlightStyle(p: Palette): Extension {
  return syntaxHighlighting(
    HighlightStyle.define([
      { tag: [tags.heading1, tags.heading2, tags.heading3, tags.heading4, tags.heading5, tags.heading6],
        color: p.heading, fontWeight: "bold" },
      { tag: tags.strong, color: p.strong, fontWeight: "bold" },
      { tag: tags.emphasis, fontStyle: "italic" },
      { tag: tags.strikethrough, textDecoration: "line-through", color: p.fgMuted },
      { tag: tags.link, color: p.link, textDecoration: "underline" },
      { tag: tags.url, color: p.url },
      { tag: tags.labelName, color: p.link },
      { tag: tags.quote, color: p.quote, fontStyle: "italic" },
      { tag: tags.monospace, color: p.code, backgroundColor: p.codeBg },
      { tag: tags.processingInstruction, color: p.markup },
      { tag: tags.contentSeparator, color: p.markup },
      { tag: [tags.comment, tags.lineComment, tags.blockComment], color: p.comment, fontStyle: "italic" },
      { tag: tags.string, color: p.code },
      { tag: tags.escape, color: p.markup },
      { tag: tags.invalid, color: "#f44747" },
    ])
  );
}

export function getEditorTheme(name: EditorThemeName): Extension {
  const p = palettes[name] ?? palettes.dark;
  return [buildChromeTheme(p), buildHighlightStyle(p)];
}
