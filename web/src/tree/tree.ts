export interface FileEntry {
  path: string;
  name: string;
  isDir: boolean;
  children?: FileEntry[];
}

export class FileTree {
  private container: HTMLElement;
  private activePath: string | null = null;
  private onFileSelect: (entry: FileEntry) => void;

  constructor(container: HTMLElement, onFileSelect: (entry: FileEntry) => void) {
    this.container = container;
    this.onFileSelect = onFileSelect;
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
    const title = document.createElement("div");
    title.className = "panel-title";
    title.textContent = "Fichiers";
    this.container.appendChild(title);
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

  private renderList(entries: FileEntry[]): HTMLUListElement {
    const ul = document.createElement("ul");
    ul.className = "tree-list";

    for (const entry of entries) {
      const li = document.createElement("li");
      if (entry.isDir) {
        li.className = "tree-dir";
        const inner = document.createElement("div");
        inner.className = "tree-item-inner";
        inner.innerHTML = `<span class="icon">▸</span><span>${entry.name}</span>`;
        li.appendChild(inner);
        if (entry.children && entry.children.length > 0) {
          li.appendChild(this.renderList(entry.children));
        }
      } else {
        const item = document.createElement("div");
        item.className = "tree-item";
        item.dataset.path = entry.path;
        item.innerHTML = `<span class="icon">📄</span><span>${entry.name}</span>`;
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
