export interface FileEntry {
  path: string;
  name: string;
  isDir: boolean;
  children?: FileEntry[];
}

const FILE_ICON_SVG = `<svg class="tree-file-icon" viewBox="0 0 12 14" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M1 1h7l3 3v9H1V1z" stroke="currentColor" stroke-width="1.2" stroke-linejoin="round"/><path d="M8 1v3h3" stroke="currentColor" stroke-width="1.2" stroke-linejoin="round"/></svg>`;

export class FileTree {
  private container: HTMLElement;
  private activePath: string | null = null;
  private onFileSelect: (entry: FileEntry) => void;
  private onFileDelete?: (entry: FileEntry) => void;
  private onCreateFile?: (dirPath: string) => void;
  private collapsedDirs: Set<string> = new Set();

  constructor(
    container: HTMLElement,
    onFileSelect: (entry: FileEntry) => void,
    onFileDelete?: (entry: FileEntry) => void,
    onCreateFile?: (dirPath: string) => void,
  ) {
    this.container = container;
    this.onFileSelect = onFileSelect;
    this.onFileDelete = onFileDelete;
    this.onCreateFile = onCreateFile;
  }

  setFiles(files: FileEntry[]): void {
    this.container.innerHTML = "";
    if (!files || files.length === 0) {
      const empty = document.createElement("div");
      empty.className = "tree-empty";
      empty.textContent = "Aucun fichier .md trouvé";
      this.container.appendChild(empty);
      return;
    }

    const header = document.createElement("div");
    header.className = "tree-panel-header";

    const title = document.createElement("div");
    title.className = "panel-title";
    title.textContent = "Fichiers";
    header.appendChild(title);

    if (this.onCreateFile) {
      const btn = document.createElement("button");
      btn.className = "tree-create-btn";
      btn.title = "Nouveau fichier";
      btn.textContent = "+";
      btn.addEventListener("click", () => this.onCreateFile!(""));
      header.appendChild(btn);
    }

    this.container.appendChild(header);
    this.container.appendChild(this.renderList(files));
  }

  setActivePath(path: string | null): void {
    if (this.activePath === path) return;
    this.activePath = path;
    this.container.querySelectorAll(".tree-item").forEach((el) => {
      const item = el as HTMLElement;
      if (item.dataset.path === path) {
        item.classList.add("active");
      } else {
        item.classList.remove("active");
      }
    });
  }

  private toggleDir(dirKey: string, li: HTMLElement): void {
    const childList = li.querySelector(":scope > .tree-list") as HTMLElement | null;
    const arrow = li.querySelector(":scope > .tree-item-inner .tree-dir-arrow") as HTMLElement | null;
    if (!childList) return;

    if (this.collapsedDirs.has(dirKey)) {
      this.collapsedDirs.delete(dirKey);
      childList.style.display = "";
      arrow?.classList.remove("collapsed");
    } else {
      this.collapsedDirs.add(dirKey);
      childList.style.display = "none";
      arrow?.classList.add("collapsed");
    }
  }

  private renderList(entries: FileEntry[]): HTMLUListElement {
    const ul = document.createElement("ul");
    ul.className = "tree-list";

    for (const entry of entries) {
      const li = document.createElement("li");

      if (entry.isDir) {
        li.className = "tree-dir";
        const inner = document.createElement("div");
        inner.className = "tree-item-inner";
        const isCollapsed = this.collapsedDirs.has(entry.path);
        inner.innerHTML = `<span class="tree-dir-arrow${isCollapsed ? " collapsed" : ""}"></span><span class="tree-dir-name">${entry.name}</span>`;

        if (entry.children && entry.children.length > 0) {
          inner.style.cursor = "pointer";
          inner.addEventListener("click", () => this.toggleDir(entry.path, li));
          const childList = this.renderList(entry.children);
          if (isCollapsed) childList.style.display = "none";
          li.appendChild(inner);
          li.appendChild(childList);
        } else {
          li.appendChild(inner);
        }
      } else {
        const item = document.createElement("div");
        item.className = "tree-item";
        item.dataset.path = entry.path;

        const iconSpan = document.createElement("span");
        iconSpan.innerHTML = FILE_ICON_SVG;
        item.appendChild(iconSpan);

        const nameSpan = document.createElement("span");
        nameSpan.className = "tree-item-name";
        nameSpan.textContent = entry.name;
        item.appendChild(nameSpan);

        if (this.onFileDelete) {
          const delBtn = document.createElement("button");
          delBtn.className = "tree-item-delete";
          delBtn.title = "Supprimer";
          delBtn.textContent = "×";
          delBtn.addEventListener("click", (e) => {
            e.stopPropagation();
            this.onFileDelete!(entry);
          });
          item.appendChild(delBtn);
        }

        if (entry.path === this.activePath) {
          item.classList.add("active");
        }
        item.addEventListener("click", () => {
          this.activePath = entry.path;
          this.container.querySelectorAll(".tree-item").forEach((el) =>
            el.classList.remove("active")
          );
          item.classList.add("active");
          this.onFileSelect(entry);
        });
        li.appendChild(item);
      }
      ul.appendChild(li);
    }
    return ul;
  }
}
