import { rpc, bus } from "../bridge";

export class Preview {
  private currentRender: string | null = null;
  private onReady: ((fileId: string) => void) | null = null;
  private onError: ((fileId: string, err: string) => void) | null = null;
  private readyListener: ((data: unknown) => void) | null = null;
  private errorListener: ((data: unknown) => void) | null = null;

  constructor(
    private frame: HTMLIFrameElement,
    private spinner: HTMLElement
  ) {}

  async render(fileId: string): Promise<void> {
    this.spinner.classList.remove("hidden");
    this.frame.src = "";

    if (this.readyListener) {
      bus.off("pdf-ready", this.readyListener);
    }
    if (this.errorListener) {
      bus.off("pdf-error", this.errorListener);
    }

    this.currentRender = fileId;

    return new Promise<void>((resolve, reject) => {
      this.readyListener = (data: unknown) => {
        const d = data as { fileId: string };
        if (d.fileId === fileId) {
          this.spinner.classList.add("hidden");
          if (this.currentRender === fileId) {
            this.frame.src = `${window.location.origin}/pdf-viewer.html?fileId=${encodeURIComponent(fileId)}`;
          }
          if (this.onReady) this.onReady(fileId);
          resolve();
        }
      };

      this.errorListener = (data: unknown) => {
        const d = data as { fileId: string; error: string; path: string };
        if (d.fileId === fileId) {
          this.spinner.classList.add("hidden");
          if (this.currentRender === fileId) {
            this.frame.src = "";
          }
          if (this.onError) this.onError(fileId, d.error);
        }
      };

      bus.on("pdf-ready", this.readyListener);
      bus.on("pdf-error", this.errorListener);

      rpc("document.renderPDF", { fileId }).catch((err) => {
        this.spinner.classList.add("hidden");
        if (this.currentRender === fileId) {
          this.frame.src = "";
        }
        reject(err);
      });
    });
  }

  clear(): void {
    this.currentRender = null;
    this.frame.src = "";
  }
}
