type Listener = (data: unknown) => void;

class EventBus {
  private listeners = new Map<string, Listener[]>();

  on(event: string, cb: Listener): () => void {
    if (!this.listeners.has(event)) {
      this.listeners.set(event, []);
    }
    this.listeners.get(event)!.push(cb);
    return () => this.off(event, cb);
  }

  off(event: string, cb: Listener): void {
    const list = this.listeners.get(event);
    if (!list) return;
    this.listeners.set(event, list.filter((l) => l !== cb));
  }

  emit(event: string, data: unknown): void {
    this.listeners.get(event)?.forEach((cb) => cb(data));
  }
}

export const bus = new EventBus();
(window as any).__bus = bus;

interface RpcResponse<T> {
  ok: boolean;
  value?: T;
  error?: string;
}

// Attend que lorca ait bindé window.rpc (race condition au démarrage).
export function waitForBridge(): Promise<void> {
  return new Promise((resolve) => {
    const check = (): void => {
      if (typeof (window as any).rpc === "function") {
        resolve();
      } else {
        setTimeout(check, 50);
      }
    };
    check();
  });
}

export async function rpc<T = unknown>(
  method: string,
  params: unknown = {}
): Promise<T> {
  const raw: string = await (window as any).rpc(method, JSON.stringify(params));
  const res: RpcResponse<T> = JSON.parse(raw);
  if (!res.ok) {
    throw new Error(res.error ?? "rpc error");
  }
  return res.value as T;
}
