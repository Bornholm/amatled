export type ToastType = "info" | "success" | "error" | "warning";

class ToastManager {
  private container: HTMLElement;

  constructor() {
    this.container = document.createElement("div");
    this.container.id = "toast-container";
    document.body.appendChild(this.container);
  }

  show(message: string, type: ToastType = "info", duration = 4000): void {
    const el = document.createElement("div");
    el.className = `toast toast--${type}`;
    el.setAttribute("role", "alert");
    el.textContent = message;
    this.container.appendChild(el);

    requestAnimationFrame(() => el.classList.add("toast--visible"));

    const dismiss = (): void => {
      el.classList.remove("toast--visible");
      el.addEventListener("transitionend", () => el.remove(), { once: true });
    };

    setTimeout(dismiss, duration);
    el.addEventListener("click", dismiss);
  }
}

export const toast = new ToastManager();
