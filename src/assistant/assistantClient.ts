import {
  BackendSrvRequest,
  FetchError,
  FetchResponse,
} from "@grafana/runtime";

export type BackendSrvFetch = (
  path: string,
  data?: BackendSrvRequest["data"],
  options?: Partial<BackendSrvRequest>
) => Promise<FetchResponse>;

export interface Message {
  id: string;
  role: "user" | "assistant";
  content: string;
}

export interface Thread {
  thread_id: string;
}

class AssistantApiError extends Error {}

export class ApiRequestError extends AssistantApiError {
  constructor(message: string, readonly cause: FetchError) {
    super(message);
  }
}

export class UserAbortError extends AssistantApiError {}

export class NoSuchThreadFoundError extends AssistantApiError {}

export class AssistantClient {
  constructor(private readonly fetch: BackendSrvFetch) {
  }

  public addThread = async (
    query: string,
    signal: AbortSignal
  ): Promise<Thread> => {
    return await this.withAbort(
      this.request<Thread>(`threads`, JSON.stringify({ query }), "POST"),
      signal
    );
  };

  public addMessage = async (
    threadId: string,
    message: string,
    query: string,
    signal: AbortSignal
  ): Promise<Message> => {
    try {
      return await this.withAbort(
        this.request<Message>(
          `threads/${threadId}/messages`,
          JSON.stringify({ thread_id: threadId, message, query }),
          "POST"
        ),
        signal
      );
    } catch (error) {
      if (error instanceof ApiRequestError) {
        const response = error.cause as FetchError<{ detail: string }>;
        if (response.status === 404 && response.data?.detail === "Thread not found") {
          throw new NoSuchThreadFoundError()
        }
      }
      throw error
    }
  };

  private async request<T>(
    path: string,
    data: BackendSrvRequest["data"],
    method: "POST" | "GET" = "GET",
    headers?: Record<string, any>
  ): Promise<T> {
    try {
       const response = await this.fetch.call(undefined, path, data, {
        responseType: "json",
        method: method,
        showErrorAlert: false,
        headers: {
          "Content-Type": "application/json",
          ...headers,
        },
      });
       return response.data
    } catch (error) {
      if (error instanceof Error) {
        throw error
      }
      return this.handleError(error as FetchError);
    }
  }

  private handleError<T>(error: FetchError): T {
    let message = "Unknown error"
    if (typeof error.data === "string") {
      message = error.data
    } else if (error.data?.detail) {
      message = error.data.detail
    }
    throw new ApiRequestError(message, error)
  }

  private withAbort<T>(promise: Promise<T>, signal: AbortSignal): Promise<T> {
    const abort = new Promise<never>((_, reject) => {
      signal.addEventListener("abort", () => reject(new UserAbortError()), {
        once: true,
      });
    });
    return Promise.race([promise, abort]);
  }
}
