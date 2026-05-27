import { rpc } from "../bridge";

export class Preview {
  constructor(
    private frame: HTMLIFrameElement,
    private spinner: HTMLElement
  ) {}

  async render(fileId: string): Promise<void> {
    this.spinner.classList.remove("hidden");
    this.frame.src = "";
    try {
      await rpc("document.renderPDF", { fileId });
      await new Promise<void>((resolve) => {
        const onMessage = (e: MessageEvent) => {
          if (e.data?.type === "pdf-rendered") {
            window.removeEventListener("message", onMessage);
            resolve();
          }
        };
        window.addEventListener("message", onMessage);
        this.frame.src = `${window.location.origin}/pdf-viewer.html?fileId=${encodeURIComponent(fileId)}`;
      });
    } catch (err) {
      this.frame.src = "";
      throw err;
    } finally {
      this.spinner.classList.add("hidden");
    }
  }

  clear(): void {
    this.frame.src = "";
  }
}
