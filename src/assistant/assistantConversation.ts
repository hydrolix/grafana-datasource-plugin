import { createStore } from "zustand";
import { v4 as uuid } from "uuid";
import { BackendSrvRequest, FetchResponse } from "@grafana/runtime";
import { AssistantClient, UserAbortError, NoSuchThreadFoundError } from "./assistantClient";

export interface AssistantMessage {
  id: string;
  role: "user" | "assistant";
  status: "sending" | "sent" | "receiving" | "received" | "failed";
  content: string;
}

export interface AssistantConversationState {
  threadId: string;
  messages: AssistantMessage[];
  thinking: boolean;
  errorMessage?: string;
}

export interface AssistantConversationStateActions {
  addMessage: (message: AssistantMessage) => void;
  updateMessage: (message: AssistantMessage) => void;
  updateMessageById: (
    id: string,
    transform: (m: AssistantMessage) => AssistantMessage
  ) => void;
  removeMessageById: (id: string) => void;
  getMessageById: (id: string) => AssistantMessage;
  setThinking: (thinking: boolean) => void;
  setThreadId: (threadId: string) => void;
  getThreadId: () => string;
  setErrorMessage: (message: string) => void;
  reset: () => void;
}

export const createAssistantConversationStateStore = (
  messages: AssistantMessage[]
) =>
  createStore<AssistantConversationState & AssistantConversationStateActions>(
    (set, get) => ({
      threadId: "",
      messages: messages,
      thinking: false,
      addMessage: (m) =>
        set((s) => ({ messages: [...s.messages, m], errorMessage: undefined })),
      updateMessage: (n) =>
        set((state) => {
          const exists = state.messages.some((m) => m.id === n.id);
          if (!exists) {
            throw new Error(`No message with id [${n.id}] found`);
          }
          return {
            messages: state.messages.map((m) => (m.id === n.id ? n : m)),
          };
        }),
      updateMessageById: (
        id: string,
        transform: (m: AssistantMessage) => AssistantMessage
      ) =>
        set((state) => {
          let updated = false;
          let messages = state.messages.map((m) => {
            if (m.id === id) {
              const n = transform(m);
              updated = true;
              return n;
            }
            return m;
          });
          if (!updated) {
            throw new Error(`No message with id [${id}] found`);
          }
          return { messages: messages };
        }),
      getMessageById: (id) => {
        let message = get().messages.find((m) => m.id === id);
        if (!message) {
          throw new Error(`No message with id [${id}] found`);
        }
        return message;
      },
      removeMessageById: (id) =>
        set((state) => ({
          messages: state.messages.filter((m) => m.id !== id),
        })),
      setThreadId: (id: string) => set({ threadId: id }),
      getThreadId: () => get().threadId,
      setThinking: (t) => set({ thinking: t }),
      reset: () =>
        set({
          thinking: false,
          messages: [],
          threadId: "",
          errorMessage: undefined,
        }),
      setErrorMessage: (message) => set({ errorMessage: message }),
    })
  );

export type AssistantConversationStateStore = ReturnType<
  typeof createAssistantConversationStateStore
>;

export interface AssistantConfig {
  fetch: (
    path: string,
    data?: BackendSrvRequest["data"],
    options?: Partial<BackendSrvRequest>
  ) => Promise<FetchResponse>;
}

export class AssistantConversation {
  private client: AssistantClient;
  private abortController: AbortController | null = null;

  constructor(
    private readonly config: AssistantConfig,
    private readonly stateStore: AssistantConversationStateStore
  ) {
    this.client = new AssistantClient(this.config.fetch);
  }

  private state() {
    return this.stateStore.getState();
  }

  async addMessage(message: string, query: string): Promise<void> {
    //avoid sending message twice
    if (this.state().thinking) {
      return;
    }
    this.state().setThinking(true);

    this.abortController?.abort();
    this.abortController = new AbortController();

    const questionMessage: AssistantMessage = {
      id: uuid(),
      role: "user",
      status: "sending",
      content: message,
    };
    this.state().addMessage(questionMessage);

    let answerMessage: AssistantMessage = {
      id: uuid(),
      role: "assistant",
      status: "receiving",
      content: "",
    };
    this.state().addMessage(answerMessage);

    try {
      let threadId = await this.getOrAddThreadId(query, this.abortController.signal);

      const answer = await this.client.addMessage(
        threadId,
        message,
        query,
        this.abortController.signal
      );

      this.state().updateMessageById(questionMessage.id, (m) => ({
        ...m,
        status: "sent",
      }));

      this.state().updateMessageById(answerMessage.id, (m) => ({
        ...m,
        id: answer.id,
        status: "received",
        content: answer.content!,
      }));
    } catch (e) {
      this.state().removeMessageById(answerMessage.id);
      if (e instanceof UserAbortError) {
        this.state().removeMessageById(questionMessage.id);
      } else {
        this.state().updateMessageById(questionMessage.id, (m) => ({
          ...m,
          status: "failed",
        }));
        if (e instanceof NoSuchThreadFoundError) {
          this.state().setErrorMessage("Conversation unavailable, reset it to continue");
        } else {
          this.state().setErrorMessage(e instanceof Error ? e.message : String(e));
        }
        throw e;
      }
    } finally {
      this.abortController = null;
      this.state().setThinking(false);
    }
  }

  private async getOrAddThreadId(query: string, signal: AbortSignal) {
    let threadId = this.state().getThreadId();
    if (threadId === null || threadId === "") {
      threadId = await this.client.addThread(query, signal).then(t => t.thread_id)
      this.state().setThreadId(threadId);
    }
    return threadId;
  }

  abort() {
    this.abortController?.abort();
    this.abortController = null;
    this.stateStore.getState().setThinking(false);
  }

  async reset() {
    this.abort();
    this.stateStore.getState().reset();
  }
}

export function createAssistantConversation(
  config: AssistantConfig,
  stateStore: AssistantConversationStateStore
): AssistantConversation {
  return new AssistantConversation(config, stateStore);
}
