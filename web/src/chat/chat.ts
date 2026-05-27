import { rpc, bus } from "../bridge";
import { v4 as uuidv4 } from "uuid";

export type MessageRole = "user" | "assistant";

export interface ChatMessage {
  id: string;
  role: MessageRole;
  content: string;
  toolCalls?: ToolCallInfo[];
  isStreaming?: boolean;
  isError?: boolean;
}

interface ToolCallInfo {
  id: string;
  name: string;
  done: boolean;
  result?: string;
}

// Types d'événements reçus du backend
interface AgentTextDelta { Delta: string }
interface AgentToolCallStart { ID: string; Name: string; Parameters: unknown }
interface AgentToolCallDone { ID: string; Name: string; Result: string }
interface AgentComplete { Message: string }
interface AgentError { Message: string }

export class Chat {
  private messages: ChatMessage[] = [];
  private isRunning = false;
  private activeFileId: string | null = null;

  constructor(
    private messagesEl: HTMLElement,
    private inputEl: HTMLTextAreaElement,
    private sendBtn: HTMLButtonElement,
    private cancelBtn: HTMLButtonElement | null,
  ) {
    this.bindEvents();
    this.listenAgentEvents();
  }

  setActiveFile(fileId: string | null): void {
    this.activeFileId = fileId;
    this.inputEl.disabled = fileId === null;
    this.sendBtn.disabled = fileId === null || this.isRunning;
    if (!fileId) {
      this.inputEl.placeholder = "Ouvrez un fichier pour utiliser l'assistant…";
    } else {
      this.inputEl.placeholder = "Votre message… (Entrée pour envoyer, Maj+Entrée pour retour à la ligne)";
    }
  }

  private bindEvents(): void {
    this.sendBtn.addEventListener("click", () => this.send());
    this.inputEl.addEventListener("keydown", (e) => {
      if (e.key === "Enter" && !e.shiftKey && !e.ctrlKey && !e.metaKey) {
        // Enter seul → envoyer
        e.preventDefault();
        this.send();
      }
      // Shift+Enter → newline natif (comportement par défaut du textarea)
    });
    if (this.cancelBtn) {
      this.cancelBtn.addEventListener("click", () => this.cancel());
    }
  }

  private listenAgentEvents(): void {
    bus.on("agent.event", (data) => {
      const ev = data as { type: string; data: unknown };
      switch (ev.type) {
        case "text_delta": {
          const d = ev.data as AgentTextDelta;
          this.appendDelta(d.Delta);
          break;
        }
        case "tool_call_start": {
          const d = ev.data as AgentToolCallStart;
          this.addToolCall(d.ID, d.Name);
          break;
        }
        case "tool_call_done": {
          const d = ev.data as AgentToolCallDone;
          this.finishToolCall(d.ID, d.Result);
          break;
        }
        case "complete": {
          const d = ev.data as AgentComplete;
          this.finalizeAssistantMessage(d.Message);
          this.setRunning(false);
          break;
        }
        case "budget_exceeded": {
          this.finalizeAssistantMessage("(Budget d'itérations dépassé)");
          this.setRunning(false);
          break;
        }
        case "error": {
          const d = ev.data as AgentError;
          this.addErrorMessage(d.Message);
          this.setRunning(false);
          break;
        }
      }
    });
  }

  private async send(): Promise<void> {
    if (!this.activeFileId || this.isRunning) return;
    const text = this.inputEl.value.trim();
    if (!text) return;

    this.inputEl.value = "";
    this.addUserMessage(text);
    this.addAssistantMessage("", true);
    this.setRunning(true);

    const aiMsgId = uuidv4();

    try {
      await rpc("chat.sendMessage", {
        fileId: this.activeFileId,
        message: text,
        aiMessageId: aiMsgId,
      });
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      this.finalizeAssistantMessage("");
      this.addErrorMessage(msg);
      this.setRunning(false);
    }
  }

  private async cancel(): Promise<void> {
    try {
      await rpc("chat.cancel", {});
    } catch {
      // silencieux
    }
    this.finalizeAssistantMessage("(Annulé par l'utilisateur)");
    this.setRunning(false);
  }

  clearConversation(): void {
    if (this.isRunning) return;
    this.messages = [];
    this.messagesEl.innerHTML = "";
  }

  private setRunning(running: boolean): void {
    this.isRunning = running;
    this.sendBtn.disabled = running || !this.activeFileId;
    this.inputEl.disabled = running || !this.activeFileId;
    if (this.cancelBtn) {
      this.cancelBtn.style.display = running ? "inline-flex" : "none";
    }
  }

  private addUserMessage(text: string): void {
    const msg: ChatMessage = { id: uuidv4(), role: "user", content: text };
    this.messages.push(msg);
    this.renderMessage(msg);
  }

  private addAssistantMessage(text: string, streaming = false): ChatMessage {
    const msg: ChatMessage = {
      id: uuidv4(),
      role: "assistant",
      content: text,
      toolCalls: [],
      isStreaming: streaming,
    };
    this.messages.push(msg);
    this.renderMessage(msg);
    return msg;
  }

  private addErrorMessage(text: string): void {
    const msg: ChatMessage = {
      id: uuidv4(),
      role: "assistant",
      content: text,
      isError: true,
    };
    this.messages.push(msg);
    this.renderMessage(msg);
  }

  private currentAssistantMsg(): ChatMessage | null {
    for (let i = this.messages.length - 1; i >= 0; i--) {
      if (this.messages[i].role === "assistant" && this.messages[i].isStreaming) {
        return this.messages[i];
      }
    }
    return null;
  }

  private appendDelta(delta: string): void {
    const msg = this.currentAssistantMsg();
    if (!msg) return;
    msg.content += delta;
    this.updateMessageEl(msg);
  }

  private addToolCall(id: string, name: string): void {
    const msg = this.currentAssistantMsg();
    if (!msg) return;
    msg.toolCalls = msg.toolCalls ?? [];
    msg.toolCalls.push({ id, name, done: false });
    this.updateMessageEl(msg);
  }

  private finishToolCall(id: string, result: string): void {
    const msg = this.currentAssistantMsg();
    if (!msg) return;
    const tc = msg.toolCalls?.find((t) => t.id === id);
    if (tc) {
      tc.done = true;
      tc.result = result;
    }
    this.updateMessageEl(msg);
  }

  private finalizeAssistantMessage(finalText?: string): void {
    const msg = this.currentAssistantMsg();
    if (!msg) return;
    if (finalText !== undefined && finalText !== "") {
      msg.content = finalText;
    }
    msg.isStreaming = false;
    this.updateMessageEl(msg);
  }

  private renderMessage(msg: ChatMessage): void {
    const el = this.buildMessageEl(msg);
    el.dataset.msgId = msg.id;
    this.messagesEl.appendChild(el);
    this.scrollToBottom();
  }

  private updateMessageEl(msg: ChatMessage): void {
    const existing = this.messagesEl.querySelector(`[data-msg-id="${msg.id}"]`);
    if (!existing) return;
    const el = this.buildMessageEl(msg);
    el.dataset.msgId = msg.id;
    existing.replaceWith(el);
    this.scrollToBottom();
  }

  private buildMessageEl(msg: ChatMessage): HTMLElement {
    const el = document.createElement("div");
    el.className = `chat-message chat-message--${msg.role}`;
    if (msg.isError) el.classList.add("chat-message--error");

    // Bulle principale de texte
    const bubble = document.createElement("div");
    bubble.className = "chat-bubble";
    if (msg.isStreaming && msg.content === "" && !msg.toolCalls?.length) {
      bubble.innerHTML = '<div class="chat-thinking"><span></span><span></span><span></span></div>';
    } else if (msg.isStreaming) {
      bubble.textContent = msg.content;
      bubble.appendChild(document.createElement("span")).className = "chat-cursor";
    } else {
      bubble.textContent = msg.content;
    }
    el.appendChild(bubble);

    // Appels d'outils
    if (msg.toolCalls && msg.toolCalls.length > 0) {
      const toolsEl = document.createElement("div");
      toolsEl.className = "chat-tools";
      for (const tc of msg.toolCalls) {
        const tcEl = document.createElement("details");
        tcEl.className = "chat-tool-call";
        if (tc.done) tcEl.classList.add("chat-tool-call--done");

        const summary = document.createElement("summary");
        summary.textContent = tc.done
          ? `✓ ${tc.name}`
          : `⟳ ${tc.name}…`;
        tcEl.appendChild(summary);

        if (tc.result) {
          const resultEl = document.createElement("pre");
          resultEl.className = "chat-tool-result";
          resultEl.textContent = tc.result.slice(0, 500) + (tc.result.length > 500 ? "…" : "");
          tcEl.appendChild(resultEl);
        }
        toolsEl.appendChild(tcEl);
      }
      el.appendChild(toolsEl);
    }

    return el;
  }

  private scrollToBottom(): void {
    this.messagesEl.scrollTop = this.messagesEl.scrollHeight;
  }
}

// Génère un UUID v4 simple si la lib uuid n'est pas disponible
function v4(): string {
  return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    const v = c === "x" ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
}

// Override de l'import uuid pour éviter la dépendance npm
const uuidv4 = v4;
