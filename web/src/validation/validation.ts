export type DiffType = "unchanged" | "added" | "removed";

export interface DiffLine {
  type: DiffType;
  leftLine?: number;
  rightLine?: number;
  leftText: string;
  rightText: string;
}

export interface ValidationCallbacks {
  onValidate?: (fileId: string) => void | Promise<void>;
  onCancel?: (fileId: string) => void | Promise<void>;
}

interface StagedState {
  fileId: string;
  original: string;
  modified: string;
}

/**
 * Calcule un diff ligne-à-ligne entre l'original et le modifié en utilisant
 * l'algorithme de la plus longue sous-séquence commune (LCS).
 */
export function computeDiff(original: string, modified: string): DiffLine[] {
  const originalLines = original.split("\n");
  const modifiedLines = modified.split("\n");

  const n = originalLines.length;
  const m = modifiedLines.length;

  const dp: number[][] = Array.from({ length: n + 1 }, () => Array(m + 1).fill(0));

  for (let i = 1; i <= n; i++) {
    for (let j = 1; j <= m; j++) {
      if (originalLines[i - 1] === modifiedLines[j - 1]) {
        dp[i][j] = dp[i - 1][j - 1] + 1;
      } else {
        dp[i][j] = Math.max(dp[i - 1][j], dp[i][j - 1]);
      }
    }
  }

  const result: DiffLine[] = [];
  let i = n;
  let j = m;

  while (i > 0 || j > 0) {
    if (i > 0 && j > 0 && originalLines[i - 1] === modifiedLines[j - 1]) {
      result.unshift({
        type: "unchanged",
        leftLine: i,
        rightLine: j,
        leftText: originalLines[i - 1],
        rightText: modifiedLines[j - 1],
      });
      i--;
      j--;
    } else if (i > 0 && (j === 0 || dp[i][j] === dp[i - 1][j])) {
      result.unshift({
        type: "removed",
        leftLine: i,
        leftText: originalLines[i - 1],
        rightText: "",
      });
      i--;
    } else if (j > 0) {
      result.unshift({
        type: "added",
        rightLine: j,
        leftText: "",
        rightText: modifiedLines[j - 1],
      });
      j--;
    }
  }

  return result;
}

/**
 * Escapes a text value for safe insertion into HTML content.
 */
function escapeHtml(text: string): string {
  const div = document.createElement("div");
  div.textContent = text;
  return div.innerHTML;
}

/**
 * Panel de validation affichant le diff Original / Modifié et permettant
 * d'éditer le contenu modifié avant de valider ou d'annuler.
 */
export class ValidationPanel {
  private container: HTMLElement;
  private originalEl: HTMLElement;
  private modifiedEl: HTMLElement;
  private validateBtn: HTMLButtonElement;
  private cancelBtn: HTMLButtonElement;

  private currentFileId: string | null = null;
  private currentOriginal = "";
  private currentModified = "";
  private state: StagedState | null = null;
  private debounceTimer: ReturnType<typeof setTimeout> | null = null;

  private onValidate?: (fileId: string) => void | Promise<void>;
  private onCancel?: (fileId: string) => void | Promise<void>;

  constructor(container: HTMLElement, callbacks: ValidationCallbacks = {}) {
    this.container = container;

    const originalEl = this.container.querySelector<HTMLElement>("#validation-original");
    const modifiedEl = this.container.querySelector<HTMLElement>("#validation-modified");
    const validateBtn = this.container.querySelector<HTMLButtonElement>("#validation-validate");
    const cancelBtn = this.container.querySelector<HTMLButtonElement>("#validation-cancel");

    if (!originalEl || !modifiedEl || !validateBtn || !cancelBtn) {
      throw new Error("ValidationPanel: missing required DOM children");
    }

    this.originalEl = originalEl;
    this.modifiedEl = modifiedEl;
    this.validateBtn = validateBtn;
    this.cancelBtn = cancelBtn;
    this.onValidate = callbacks.onValidate;
    this.onCancel = callbacks.onCancel;

    this.bindEvents();
  }

  private bindEvents(): void {
    this.validateBtn.addEventListener("click", () => {
      if (this.currentFileId && this.onValidate) {
        this.onValidate(this.currentFileId);
      }
    });

    this.cancelBtn.addEventListener("click", () => {
      if (this.currentFileId && this.onCancel) {
        this.onCancel(this.currentFileId);
      }
    });

    // Recalcul du diff en temps réel lors de l'édition du contenu modifié.
    this.modifiedEl.addEventListener("input", () => {
      this.currentModified = this.modifiedEl.innerText;
      this.renderDiff();

      // Après un court délai, on reconstruit les lignes du contenteditable
      // pour garantir une coloration cohérente après ajout/suppression de lignes.
      if (this.debounceTimer !== null) clearTimeout(this.debounceTimer);
      this.debounceTimer = setTimeout(() => {
        this.debounceTimer = null;
        // On ne reconstruit que si le nombre de lignes dans le DOM ne correspond
        // plus au nombre de lignes du contenu modifié.
        const currentLines = this.modifiedEl.children.length;
        const targetLines = this.currentModified === "" ? 0 : this.currentModified.split("\n").length;
        if (currentLines !== targetLines) {
          this.renderModified();
        }
        this.renderDiff();
      }, 300);
    });
  }

  /**
   * Ouvre le panel avec le contenu original et le contenu modifié.
   */
  open(fileId: string, original: string, modified: string): void {
    this.currentFileId = fileId;
    this.currentOriginal = original;
    this.currentModified = modified;
    this.state = { fileId, original, modified };

    this.renderModified();
    this.renderDiff();

    this.container.classList.remove("hidden");
  }

  /**
   * Ferme le panel et réinitialise l'état interne.
   */
  close(): void {
    this.container.classList.add("hidden");
    this.currentFileId = null;
    this.currentOriginal = "";
    this.currentModified = "";
    this.state = null;
  }

  isOpen(): boolean {
    return !this.container.classList.contains("hidden");
  }

  /**
   * Retourne le contenu actuel de la zone d'édition modifiée.
   */
  getModifiedContent(): string {
    return this.currentModified;
  }

  getFileId(): string | null {
    return this.currentFileId;
  }

  getOriginalContent(): string {
    return this.currentOriginal;
  }

  /**
   * Définit le nouveau contenu modifié (par exemple après un appel backend).
   */
  setModifiedContent(content: string): void {
    this.currentModified = content;
    this.renderModified();
    this.renderDiff();
  }

  private renderModified(): void {
    // On représente chaque ligne par un <div> pour que l'édition soit naturelle
    // et que innerText produise des sauts de ligne.
    this.modifiedEl.innerHTML = this.currentModified
      .split("\n")
      .map((line) => `<div>${escapeHtml(line)}</div>`)
      .join("");
  }

  private renderDiff(): void {
    const diff = computeDiff(this.currentOriginal, this.currentModified);

    this.originalEl.innerHTML = "";

    for (const line of diff) {
      const row = document.createElement("div");
      row.className = `validation-line validation-line--${line.type}`;

      const lineNum = document.createElement("span");
      lineNum.className = "validation-line-number";
      lineNum.textContent = line.leftLine !== undefined ? String(line.leftLine) : "";

      const text = document.createElement("span");
      text.className = "validation-line-text";
      text.textContent = line.type === "removed" || line.type === "unchanged"
        ? line.leftText
        : "";

      row.appendChild(lineNum);
      row.appendChild(text);
      this.originalEl.appendChild(row);
    }

    // Recolorer la colonne droite sans perdre le contenu éditable.
    // On garde les divs existantes et on ajuste leurs classes pour refléter le diff.
    const rightLines = diff.filter((line) => line.type === "added" || line.type === "unchanged");
    const modifiedDivs = Array.from(this.modifiedEl.children) as HTMLDivElement[];

    for (let idx = 0; idx < rightLines.length; idx++) {
      const line = rightLines[idx];
      const div = modifiedDivs[idx];
      if (!div) continue;
      div.className = `validation-line validation-line--${line.type}`;
    }
  }
}
