import { rpc } from "../bridge";

export interface LLMSettings {
  provider: string;
  baseURL: string;
  apiKey: string;
  model: string;
  maxIterations: number;
  maxTokens: number;
}

export interface Profile {
  name: string;
  llm: LLMSettings;
  systemPrompt: string;
}

export interface RenderPreset {
  name: string;
  renderConfig: string;
  renderConfigUsername: string;
  renderConfigPassword: string;
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
  private profileSelect: HTMLSelectElement;
  private profileNewBtn: HTMLButtonElement;
  private profileDeleteBtn: HTMLButtonElement;
  private presetSelect: HTMLSelectElement;
  private presetNewBtn: HTMLButtonElement;
  private presetDeleteBtn: HTMLButtonElement;
  private presetForm: HTMLElement;
  private presetEmpty: HTMLElement;

  private profiles: Profile[] = [];
  private currentProfileName = "";
  private renderPresets: RenderPreset[] = [];
  private currentPresetName = "";
  private activeTopTab = "profiles";

  constructor(private container: HTMLElement) {
    this.modal = container.querySelector<HTMLElement>("#settings-modal")!;
    this.form = container.querySelector<HTMLFormElement>("#settings-form")!;
    this.testBtn = container.querySelector<HTMLButtonElement>("#settings-test-btn")!;
    this.testStatus = container.querySelector<HTMLElement>("#settings-test-status")!;
    this.saveBtn = container.querySelector<HTMLButtonElement>("#settings-save-btn")!;
    this.updateBtn = container.querySelector<HTMLButtonElement>("#settings-update-btn")!;
    this.updateStatus = container.querySelector<HTMLElement>("#s-update-status")!;
    this.versionLabel = container.querySelector<HTMLElement>("#s-version")!;
    this.profileSelect = container.querySelector<HTMLSelectElement>("#s-profile-select")!;
    this.profileNewBtn = container.querySelector<HTMLButtonElement>("#s-profile-new")!;
    this.profileDeleteBtn = container.querySelector<HTMLButtonElement>("#s-profile-delete")!;
    this.presetSelect = container.querySelector<HTMLSelectElement>("#s-preset-select")!;
    this.presetNewBtn = container.querySelector<HTMLButtonElement>("#s-preset-new")!;
    this.presetDeleteBtn = container.querySelector<HTMLButtonElement>("#s-preset-delete")!;
    this.presetForm = container.querySelector<HTMLElement>("#s-preset-form")!;
    this.presetEmpty = container.querySelector<HTMLElement>("#s-preset-empty")!;

    this.bindEvents();
  }

  async open(): Promise<void> {
    try {
      const [profiles, renderPresets, general] = await Promise.all([
        rpc<Profile[]>("settings.listProfiles", {}),
        rpc<RenderPreset[]>("settings.listRenderPresets", {}),
        rpc<{ normalizeOnSave: boolean; autoUpdate: boolean; version: string; activeProfile?: string }>("settings.get", {}),
      ]);
      this.profiles = profiles ?? [];
      this.renderPresets = renderPresets ?? [];
      this.rebuildProfileSelect(general.activeProfile);
      this.fillFormFromCurrentProfile();
      this.rebuildPresetSelect();
      this.fillFormFromCurrentPreset();
      (this.form.querySelector("#s-normalize-on-save") as HTMLInputElement).checked =
        general.normalizeOnSave !== false;
      (this.form.querySelector("#s-auto-update") as HTMLInputElement).checked =
        general.autoUpdate !== false;
      this.versionLabel.textContent = general.version || "dev";
    } catch {
      this.versionLabel.textContent = "dev";
    }
    this.testStatus.textContent = "";
    this.testStatus.className = "settings-test-status";
    this.updateStatus.textContent = "";
    this.updateStatus.className = "settings-test-status";
    this.updateBtn.textContent = "Vérifier les mises à jour";
    this.updateBtn.onclick = null;
    this.switchTopTab(this.activeTopTab);
    this.modal.classList.add("open");
    const firstInput = this.modal.querySelector<HTMLElement>(".settings-top-panel.active select, .settings-top-panel.active input");
    firstInput?.focus();
  }

  close(): void {
    this.modal.classList.remove("open");
    this.testStatus.textContent = "";
    this.testStatus.className = "settings-test-status";
    this.updateStatus.textContent = "";
    this.updateStatus.className = "settings-test-status";
  }

  private rebuildProfileSelect(activeProfileName?: string): void {
    this.profileSelect.innerHTML = "";
    for (const p of this.profiles) {
      const opt = document.createElement("option");
      opt.value = p.name;
      opt.textContent = p.name;
      this.profileSelect.appendChild(opt);
    }
    const target = activeProfileName ?? this.profiles[0]?.name ?? "";
    this.profileSelect.value = target;
    this.currentProfileName = this.profileSelect.value;
    this.profileDeleteBtn.disabled = this.profiles.length <= 1;
  }

  private fillFormFromCurrentProfile(): void {
    const p = this.profiles.find((x) => x.name === this.currentProfileName);
    if (!p) return;
    (this.form.querySelector("#s-provider") as HTMLSelectElement).value = p.llm.provider || "openai";
    (this.form.querySelector("#s-baseurl") as HTMLInputElement).value = p.llm.baseURL || "";
    (this.form.querySelector("#s-apikey") as HTMLInputElement).value = p.llm.apiKey || "";
    (this.form.querySelector("#s-model") as HTMLInputElement).value = p.llm.model || "";
    (this.form.querySelector("#s-max-iter") as HTMLInputElement).value = String(p.llm.maxIterations || 20);
    (this.form.querySelector("#s-max-tokens") as HTMLInputElement).value = String(p.llm.maxTokens || 80000);
    (this.form.querySelector("#s-system-prompt") as HTMLTextAreaElement).value = p.systemPrompt || "";
  }

  private collectCurrentProfile(): Profile {
    return {
      name: this.currentProfileName,
      llm: {
        provider: (this.form.querySelector("#s-provider") as HTMLSelectElement).value,
        baseURL: (this.form.querySelector("#s-baseurl") as HTMLInputElement).value.trim(),
        apiKey: (this.form.querySelector("#s-apikey") as HTMLInputElement).value.trim(),
        model: (this.form.querySelector("#s-model") as HTMLInputElement).value.trim(),
        maxIterations: parseInt((this.form.querySelector("#s-max-iter") as HTMLInputElement).value, 10) || 20,
        maxTokens: parseInt((this.form.querySelector("#s-max-tokens") as HTMLInputElement).value, 10) || 80000,
      },
      systemPrompt: (this.form.querySelector("#s-system-prompt") as HTMLTextAreaElement).value.trim(),
    };
  }

  private rebuildPresetSelect(): void {
    const hasPresets = this.renderPresets.length > 0;
    this.presetSelect.style.display = hasPresets ? "" : "none";
    this.presetDeleteBtn.disabled = !hasPresets;
    this.presetEmpty.classList.toggle("hidden", hasPresets);
    this.presetForm.classList.toggle("hidden", !hasPresets);
    if (!hasPresets) {
      this.currentPresetName = "";
      return;
    }
    this.presetSelect.innerHTML = "";
    for (const rp of this.renderPresets) {
      const opt = document.createElement("option");
      opt.value = rp.name;
      opt.textContent = rp.name;
      this.presetSelect.appendChild(opt);
    }
    if (!this.renderPresets.find((rp) => rp.name === this.currentPresetName)) {
      this.currentPresetName = this.renderPresets[0].name;
    }
    this.presetSelect.value = this.currentPresetName;
  }

  private fillFormFromCurrentPreset(): void {
    const rp = this.renderPresets.find((x) => x.name === this.currentPresetName);
    (document.getElementById("s-render-config") as HTMLInputElement).value = rp?.renderConfig ?? "";
    (document.getElementById("s-render-config-username") as HTMLInputElement).value = rp?.renderConfigUsername ?? "";
    (document.getElementById("s-render-config-password") as HTMLInputElement).value = rp?.renderConfigPassword ?? "";
  }

  private collectCurrentPreset(): RenderPreset {
    return {
      name: this.currentPresetName,
      renderConfig: (document.getElementById("s-render-config") as HTMLInputElement).value.trim(),
      renderConfigUsername: (document.getElementById("s-render-config-username") as HTMLInputElement).value.trim(),
      renderConfigPassword: (document.getElementById("s-render-config-password") as HTMLInputElement).value,
    };
  }

  private switchTopTab(name: string): void {
    this.activeTopTab = name;
    this.modal.querySelectorAll<HTMLElement>(".settings-top-tab").forEach((btn) => {
      const active = btn.dataset.topTab === name;
      btn.classList.toggle("active", active);
      btn.setAttribute("aria-selected", String(active));
    });
    this.modal.querySelectorAll<HTMLElement>(".settings-top-panel").forEach((panel) => {
      panel.classList.toggle("active", panel.dataset.topPanel === name);
    });
    this.testBtn.style.display = name === "profiles" ? "" : "none";
    this.updateBtn.style.display = name === "profiles" ? "" : "none";
  }

  private bindEvents(): void {
    const closeBtn = this.modal.querySelector(".settings-close-btn");
    closeBtn?.addEventListener("click", () => this.close());

    this.modal.querySelectorAll<HTMLButtonElement>(".settings-top-tab").forEach((tab) => {
      tab.addEventListener("click", () => this.switchTopTab(tab.dataset.topTab!));
    });

    this.modal.addEventListener("click", (e) => {
      if (e.target === this.modal) this.close();
    });

    this.modal.addEventListener("keydown", (e) => {
      if (e.key === "Escape") {
        e.stopPropagation();
        this.close();
        return;
      }
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
          if (document.activeElement === first) { e.preventDefault(); last.focus(); }
        } else {
          if (document.activeElement === last) { e.preventDefault(); first.focus(); }
        }
      }
    });

    // Changement de profil sélectionné
    this.profileSelect.addEventListener("change", () => {
      this.currentProfileName = this.profileSelect.value;
      this.fillFormFromCurrentProfile();
      this.testStatus.textContent = "";
      this.testStatus.className = "settings-test-status";
    });

    // Nouveau profil
    this.profileNewBtn.addEventListener("click", async () => {
      const name = prompt("Nom du nouveau profil :");
      if (!name?.trim()) return;
      const trimmedName = name.trim();
      if (this.profiles.some((p) => p.name === trimmedName)) {
        alert(`Un profil nommé « ${trimmedName} » existe déjà.`);
        return;
      }
      const newProfile: Profile = {
        name: trimmedName,
        llm: { provider: "openai", baseURL: "https://api.openai.com/v1", apiKey: "", model: "", maxIterations: 20, maxTokens: 80000 },
        systemPrompt: "",
      };
      try {
        await rpc("settings.createProfile", newProfile);
        this.profiles.push(newProfile);
        this.rebuildProfileSelect(trimmedName);
        this.fillFormFromCurrentProfile();
      } catch (err) {
        alert("Impossible de créer le profil : " + String(err));
      }
    });

    // Changement de préset sélectionné — sauvegarder l'actuel avant de basculer
    this.presetSelect.addEventListener("change", async () => {
      if (this.currentPresetName) await this.saveCurrentPreset();
      this.currentPresetName = this.presetSelect.value;
      this.fillFormFromCurrentPreset();
    });

    // Nouveau préset
    this.presetNewBtn.addEventListener("click", async () => {
      const name = prompt("Nom du nouveau préset de rendu :");
      if (!name?.trim()) return;
      const trimmedName = name.trim();
      if (this.renderPresets.some((rp) => rp.name === trimmedName)) {
        alert(`Un préset nommé « ${trimmedName} » existe déjà.`);
        return;
      }
      const newPreset: RenderPreset = { name: trimmedName, renderConfig: "", renderConfigUsername: "", renderConfigPassword: "" };
      try {
        await rpc("settings.createRenderPreset", newPreset);
        this.renderPresets.push(newPreset);
        this.currentPresetName = trimmedName;
        this.rebuildPresetSelect();
        this.fillFormFromCurrentPreset();
      } catch (err) {
        alert("Impossible de créer le préset : " + String(err));
      }
    });

    // Supprimer préset
    this.presetDeleteBtn.addEventListener("click", async () => {
      if (this.renderPresets.length === 0) return;
      if (!confirm(`Supprimer le préset « ${this.currentPresetName} » ?`)) return;
      const nameToDelete = this.currentPresetName;
      try {
        await rpc("settings.deleteRenderPreset", { name: nameToDelete });
        this.renderPresets = this.renderPresets.filter((rp) => rp.name !== nameToDelete);
        this.currentPresetName = this.renderPresets[0]?.name ?? "";
        this.rebuildPresetSelect();
        this.fillFormFromCurrentPreset();
      } catch (err) {
        alert("Impossible de supprimer le préset : " + String(err));
      }
    });


    // Supprimer profil
    this.profileDeleteBtn.addEventListener("click", async () => {
      if (this.profiles.length <= 1) return;
      if (!confirm(`Supprimer le profil « ${this.currentProfileName} » ?`)) return;
      const nameToDelete = this.currentProfileName;
      try {
        await rpc("settings.deleteProfile", { name: nameToDelete });
        this.profiles = this.profiles.filter((p) => p.name !== nameToDelete);
        this.rebuildProfileSelect(this.profiles[0]?.name);
        this.fillFormFromCurrentProfile();
      } catch (err) {
        alert("Impossible de supprimer le profil : " + String(err));
      }
    });

    // Test connexion
    this.testBtn.addEventListener("click", async () => {
      await this.saveCurrentProfile();
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

    // Enregistrer
    this.saveBtn.addEventListener("click", async () => {
      await this.save();
      this.close();
    });

    // Vérifier mises à jour
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

    // Mettre à jour baseURL par défaut selon le provider
    const providerSelect = this.form.querySelector("#s-provider") as HTMLSelectElement;
    providerSelect.addEventListener("change", () => {
      const baseURLInput = this.form.querySelector("#s-baseurl") as HTMLInputElement;
      if (!baseURLInput.value) {
        baseURLInput.value = defaultBaseURL(providerSelect.value);
      }
    });
  }

  private async saveCurrentProfile(): Promise<void> {
    const profile = this.collectCurrentProfile();
    try {
      await rpc("settings.updateProfile", profile);
      const idx = this.profiles.findIndex((p) => p.name === profile.name);
      if (idx >= 0) this.profiles[idx] = profile;
    } catch (err) {
      console.error("saveCurrentProfile failed", err);
    }
  }

  private async saveCurrentPreset(): Promise<void> {
    if (!this.currentPresetName) return;
    const preset = this.collectCurrentPreset();
    try {
      await rpc("settings.updateRenderPreset", preset);
      const idx = this.renderPresets.findIndex((rp) => rp.name === preset.name);
      if (idx >= 0) this.renderPresets[idx] = { ...this.renderPresets[idx], ...preset };
    } catch (err) {
      console.error("saveCurrentPreset failed", err);
    }
  }

  private async save(): Promise<void> {
    const normalizeOnSave = (this.form.querySelector("#s-normalize-on-save") as HTMLInputElement).checked;
    const autoUpdate = (this.form.querySelector("#s-auto-update") as HTMLInputElement).checked;
    try {
      await Promise.all([
        this.saveCurrentProfile(),
        this.saveCurrentPreset(),
        rpc("settings.saveGeneral", { normalizeOnSave, autoUpdate }),
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
