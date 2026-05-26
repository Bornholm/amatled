import { StateEffect, StateField } from "@codemirror/state";
import { Decoration, DecorationSet, EditorView } from "@codemirror/view";

export interface SectionRange {
  startLine: number; // 0-indexed
  endLine: number;   // 0-indexed, inclusive
}

// Effect pour mettre à jour (ou effacer) la section active.
export const setSectionEffect = StateEffect.define<SectionRange | null>();

// Décoration de ligne : classe CSS appliquée à chaque ligne de la section.
const sectionLineDeco = Decoration.line({ class: "cm-active-section" });

// StateField qui maintient l'ensemble des décorations de la section active.
const sectionDecoField = StateField.define<DecorationSet>({
  create() {
    return Decoration.none;
  },
  update(deco, tr) {
    // Suivre les changements de positions
    deco = deco.map(tr.changes);

    for (const e of tr.effects) {
      if (!e.is(setSectionEffect)) continue;
      if (e.value === null) {
        deco = Decoration.none;
        break;
      }
      const { startLine, endLine } = e.value;
      const marks: ReturnType<typeof sectionLineDeco.range>[] = [];
      const totalLines = tr.state.doc.lines;

      for (let l = startLine; l <= endLine; l++) {
        const cm6Line = l + 1; // CM6 lines are 1-indexed
        if (cm6Line < 1 || cm6Line > totalLines) continue;
        const line = tr.state.doc.line(cm6Line);
        marks.push(sectionLineDeco.range(line.from));
      }
      deco = marks.length > 0 ? Decoration.set(marks, true) : Decoration.none;
    }
    return deco;
  },
  provide: (f) => EditorView.decorations.from(f),
});

export const sectionGutterExtension = [sectionDecoField];
