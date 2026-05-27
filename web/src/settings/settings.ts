import { rpc } from "../bridge";

export interface LLMSettings {
  provider: string;
  baseURL: string;
  apiKey: string;
  model: string;
  maxIterations: number;
  maxTokens: number;
}

export class SettingsModal {
  private modal: HTMLElement;
  private form: HTMLFormElement;
  private testBtn: HTMLButtonElement;
  private testStatus: HTMLElement;
  private saveBtn: HTMLButtonElement;
  private updateBtn: HTMLButtonElement;
  private updateStatus: HTMLElement;
  private versionLabel: HTMLElement;

  constructor(private container: HTMLElement) {
    this.modal = container.querySelector<HTMLElement>("#settings-modal")!;
    this.form = container.querySelector<HTMLFormElement>("#settings-form")!;
    this.testBtn = container.querySelector<HTMLButtonElement>("#settings-test-btn")!;
    this.testStatus = container.querySelector<HTMLElement>("#settings-test-status")!;
    this.saveBtn = container.querySelector<HTMLButtonElement>("#settings-save-btn")!;
    this.updateBtn = container.querySelector<HTMLButtonElement>("#settings-update-btn")!;
    this.updateStatus = container.querySelector<HTMLElement>("#s-update-status")!;
    this.versionLabel = container.querySelector<HTMLElement>("#s-version")!;

    this.bindEvents();
  }

  async open(): Promise<void> {
    try {
      const [llm, general] = await Promise.all([
        rpc<LLMSettings>("settings.getLLM", {}),
        rpc<{ normalizeOnSave: boolean; autoUpdate: boolean; renderConfig: string; renderConfigUsername: string; renderConfigPassword: string; version: string }>("settings.get", {}),
      ]);
      this.fillForm(llm);
      (this.form.querySelector("#s-normalize-on-save") as HTMLInputElement).checked =
        general.normalizeOnSave !== false;
      (this.form.querySelector("#s-auto-update") as HTMLInputElement).checked =
        general.autoUpdate !== false;
      (this.form.querySelector("#s-render-config") as HTMLInputElement).value =
        general.renderConfig || "";
      (this.form.querySelector("#s-render-config-username") as HTMLInputElement).value =
        general.renderConfigUsername || "";
      (this.form.querySelector("#s-render-config-password") as HTMLInputElement).value =
        general.renderConfigPassword || "";
      this.versionLabel.textContent = general.version || "dev";
    } catch {
      (this.form.querySelector("#s-normalize-on-save") as HTMLInputElement).checked = true;
      (this.form.querySelector("#s-auto-update") as HTMLInputElement).checked = true;
      (this.form.querySelector("#s-render-config") as HTMLInputElement).value = "";
      (this.form.querySelector("#s-render-config-username") as HTMLInputElement).value = "";
      (this.form.querySelector("#s-render-config-password") as HTMLInputElement).value = "";
      this.versionLabel.textContent = "dev";
    }
    this.updateStatus.textContent = "";
    this.updateStatus.className = "settings-test-status";
    this.updateBtn.textContent = "Vérifier les mises à jour";
    this.updateBtn.onclick = null;
    this.modal.classList.add("open");
    // Focus le premier champ interactif
    const firstInput = this.modal.querySelector<HTMLElement>("select, input, button");
    firstInput?.focus();
  }

  close(): void {
    this.modal.classList.remove("open");
    this.testStatus.textContent = "";
    this.testStatus.className = "settings-test-status";
    this.updateStatus.textContent = "";
    this.updateStatus.className = "settings-test-status";
  }

  private fillForm(s: LLMSettings): void {
    (this.form.querySelector("#s-provider") as HTMLSelectElement).value = s.provider || "openai";
    (this.form.querySelector("#s-baseurl") as HTMLInputElement).value = s.baseURL || "";
    (this.form.querySelector("#s-apikey") as HTMLInputElement).value = s.apiKey || "";
    (this.form.querySelector("#s-model") as HTMLInputElement).value = s.model || "";
    (this.form.querySelector("#s-max-iter") as HTMLInputElement).value = String(s.maxIterations || 20);
    (this.form.querySelector("#s-max-tokens") as HTMLInputElement).value = String(s.maxTokens || 80000);
  }

  private collectForm(): LLMSettings {
    return {
      provider: (this.form.querySelector("#s-provider") as HTMLSelectElement).value,
      baseURL: (this.form.querySelector("#s-baseurl") as HTMLInputElement).value.trim(),
      apiKey: (this.form.querySelector("#s-apikey") as HTMLInputElement).value.trim(),
      model: (this.form.querySelector("#s-model") as HTMLInputElement).value.trim(),
      maxIterations: parseInt((this.form.querySelector("#s-max-iter") as HTMLInputElement).value, 10) || 20,
      maxTokens: parseInt((this.form.querySelector("#s-max-tokens") as HTMLInputElement).value, 10) || 80000,
    };
  }

  private bindEvents(): void {
    const closeBtn = this.modal.querySelector(".settings-close-btn");
    closeBtn?.addEventListener("click", () => this.close());

    // Clic en dehors de la modale
    this.modal.addEventListener("click", (e) => {
      if (e.target === this.modal) this.close();
    });

    // Escape ferme la modale
    this.modal.addEventListener("keydown", (e) => {
      if (e.key === "Escape") {
        e.stopPropagation();
        this.close();
        return;
      }
      // Focus trap : Tab / Shift+Tab reste dans la modale
      if (e.key === "Tab") {
        const focusable = Array.from(
          this.modal.querySelectorAll<HTMLElement>(
            'button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
          )
        );
        if (focusable.length === 0) return;
        const first = focusable[0];
        const last = focusable[focusable.length - 1];
        if (e.shiftKey) {
          if (document.activeElement === first) {
            e.preventDefault();
            last.focus();
          }
        } else {
          if (document.activeElement === last) {
            e.preventDefault();
            first.focus();
          }
        }
      }
    });

    this.testBtn.addEventListener("click", async () => {
      await this.save();
      this.testStatus.textContent = "Test en cours…";
      this.testStatus.className = "settings-test-status";
      try {
        await rpc("settings.testLLM", {});
        this.testStatus.textContent = "✓ Connexion réussie";
        this.testStatus.className = "settings-test-status settings-test-status--ok";
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err);
        this.testStatus.textContent = "✗ " + msg;
        this.testStatus.className = "settings-test-status settings-test-status--error";
      }
    });

    this.saveBtn.addEventListener("click", async () => {
      await this.save();
      this.close();
    });

    this.updateBtn.addEventListener("click", async () => {
      this.updateStatus.textContent = "Vérification en cours…";
      this.updateStatus.className = "settings-test-status";
      try {
        const result = await rpc<{ upToDate: boolean; version?: string }>("updater.check", {});
        if (result.upToDate) {
          this.updateStatus.textContent = "✓ Déjà à jour";
          this.updateStatus.className = "settings-test-status settings-test-status--ok";
        } else {
          this.updateStatus.textContent = `Nouvelle version disponible : ${result.version}`;
          this.updateStatus.className = "settings-test-status settings-test-status--ok";
          this.updateBtn.textContent = "Installer la mise à jour";
          this.updateBtn.onclick = async () => {
            this.updateStatus.textContent = "Installation en cours…";
            this.updateStatus.className = "settings-test-status";
            try {
              await rpc("updater.apply", {});
              this.updateStatus.textContent = "✓ Mis à jour. Relancez l'application pour appliquer.";
              this.updateStatus.className = "settings-test-status settings-test-status--ok";
              this.updateBtn.disabled = true;
            } catch (err: unknown) {
              const msg = err instanceof Error ? err.message : String(err);
              this.updateStatus.textContent = "✗ " + msg;
              this.updateStatus.className = "settings-test-status settings-test-status--error";
            }
          };
        }
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err);
        this.updateStatus.textContent = "✗ " + msg;
        this.updateStatus.className = "settings-test-status settings-test-status--error";
      }
    });

    // Mettre à jour la valeur par défaut de baseURL selon le provider
    const providerSelect = this.form.querySelector("#s-provider") as HTMLSelectElement;
    providerSelect.addEventListener("change", () => {
      const baseURLInput = this.form.querySelector("#s-baseurl") as HTMLInputElement;
      if (!baseURLInput.value) {
        baseURLInput.value = defaultBaseURL(providerSelect.value);
      }
    });
  }

  private async save(): Promise<void> {
    const normalizeOnSave = (this.form.querySelector("#s-normalize-on-save") as HTMLInputElement).checked;
    const autoUpdate = (this.form.querySelector("#s-auto-update") as HTMLInputElement).checked;
    const renderConfig = (this.form.querySelector("#s-render-config") as HTMLInputElement).value.trim();
    const renderConfigUsername = (this.form.querySelector("#s-render-config-username") as HTMLInputElement).value.trim();
    const renderConfigPassword = (this.form.querySelector("#s-render-config-password") as HTMLInputElement).value;
    try {
      await Promise.all([
        rpc("settings.saveLLM", this.collectForm()),
        rpc("settings.saveGeneral", { normalizeOnSave, autoUpdate, renderConfig, renderConfigUsername, renderConfigPassword }),
      ]);
    } catch (err) {
      console.error("save settings failed", err);
    }
  }
}

function defaultBaseURL(provider: string): string {
  switch (provider) {
    case "openai": return "https://api.openai.com/v1";
    case "openrouter": return "https://openrouter.ai/api/v1";
    case "mistral": return "https://api.mistral.ai/v1";
    default: return "";
  }
}
