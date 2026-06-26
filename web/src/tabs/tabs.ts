export interface Tab {
  path: string;
  fileId: string; // = path pour l'instant ; sera distinct si renommage
  name: string;
  content: string; // cache local (source of truth = Go Stack)
  isDirty: boolean;
}

export class TabManager {
  private tabs: Tab[] = [];
  private activeTab: string | null = null;
  private tabsBar: HTMLElement;
  private onSwitch: (tab: Tab) => void;
  private onClose: (path: string) => void;

  constructor(
    tabsBar: HTMLElement,
    onSwitch: (tab: Tab) => void,
    onClose: (path: string) => void
  ) {
    this.tabsBar = tabsBar;
    this.onSwitch = onSwitch;
    this.onClose = onClose;
  }

  open(path: string, name: string, content: string): void {
    const existing = this.tabs.find((t) => t.path === path);
    if (existing) {
      this.switchTo(path);
      return;
    }
    this.tabs.push({
      path,
      fileId: path,
      name,
      content,
      isDirty: false,
    });
    this.render();
    this.switchTo(path);
  }

  switchTo(path: string): void {
    const tab = this.tabs.find((t) => t.path === path);
    if (!tab) return;
    this.activeTab = path;
    this.render();
    this.onSwitch(tab);
  }

  close(path: string): void {
    const tab = this.tabs.find((t) => t.path === path);
    if (!tab) return;
    if (tab.isDirty) {
      const confirmed = window.confirm(
        `"${tab.name}" comporte des modifications non sauvegardées.\nFermer quand même ?`
      );
      if (!confirmed) return;
    }
    const idx = this.tabs.findIndex((t) => t.path === path);
    this.tabs.splice(idx, 1);

    if (this.activeTab === path) {
      const next = this.tabs[idx] ?? this.tabs[idx - 1] ?? null;
      this.activeTab = next ? next.path : null;
      if (next) {
        this.render();
        this.onSwitch(next);
      } else {
        this.render();
        this.onClose(path);
      }
    } else {
      this.render();
    }
  }

  setDirty(path: string, dirty: boolean): void {
    const tab = this.tabs.find((t) => t.path === path);
    if (!tab || tab.isDirty === dirty) return;
    tab.isDirty = dirty;
    this.render();
  }

  updateContent(path: string, content: string): void {
    const tab = this.tabs.find((t) => t.path === path);
    if (tab) tab.content = content;
  }

  getActive(): Tab | null {
    return this.tabs.find((t) => t.path === this.activeTab) ?? null;
  }

  getTab(path: string): Tab | undefined {
    return this.tabs.find((t) => t.path === path);
  }

  hasUnsaved(): boolean {
    return this.tabs.some((t) => t.isDirty);
  }

  private render(): void {
    this.tabsBar.innerHTML = "";
    for (const tab of this.tabs) {
      const el = document.createElement("div");
      el.className = "tab" + (tab.path === this.activeTab ? " active" : "");

      const dirty = document.createElement("span");
      dirty.className = "tab-dirty";
      dirty.style.visibility = tab.isDirty ? "visible" : "hidden";

      const label = document.createElement("span");
      label.textContent = tab.name;

      const close = document.createElement("button");
      close.className = "tab-close";
      close.innerHTML = "×";
      close.title = "Fermer";
      close.addEventListener("click", (e) => {
        e.stopPropagation();
        this.close(tab.path);
      });

      el.appendChild(dirty);
      el.appendChild(label);
      el.appendChild(close);
      el.addEventListener("click", () => this.switchTo(tab.path));
      el.addEventListener("auxclick", (e) => {
        if (e.button === 1) {
          e.preventDefault();
          e.stopPropagation();
          this.close(tab.path);
        }
      });
      this.tabsBar.appendChild(el);
    }
  }
}
