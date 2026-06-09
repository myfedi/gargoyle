import { getApiBaseUrl, trimTrailingSlash } from "@/lib/config";
import type { MastodonNotification } from "@/types/mastodon";

export type NotificationStreamHandlers = {
  onNotification: (notification: MastodonNotification) => void;
  onError?: (error: Error) => void;
};

export function startNotificationStream(accessToken: string, handlers: NotificationStreamHandlers) {
  const controller = new AbortController();
  let retryTimeout: number | undefined;
  let stopped = false;

  async function connect() {
    try {
      const response = await fetch(`${trimTrailingSlash(getApiBaseUrl())}/api/v1/streaming/user/notification`, {
        headers: { Accept: "text/event-stream", Authorization: `Bearer ${accessToken}` },
        credentials: "same-origin",
        signal: controller.signal,
      });
      if (!response.ok || !response.body) {
        throw new Error(`Notification stream failed with status ${response.status}`);
      }
      await readServerSentEvents(response.body, (event, data) => {
        if (event !== "notification" || !data) {
          return;
        }
        handlers.onNotification(JSON.parse(data) as MastodonNotification);
      });
    } catch (error) {
      if (stopped || controller.signal.aborted) {
        return;
      }
      handlers.onError?.(error instanceof Error ? error : new Error("Notification stream disconnected."));
      retryTimeout = globalThis.setTimeout(connect, 3000);
    }
  }

  void connect();

  return () => {
    stopped = true;
    controller.abort();
    if (retryTimeout) {
      globalThis.clearTimeout(retryTimeout);
    }
  };
}

async function readServerSentEvents(body: ReadableStream<Uint8Array>, onEvent: (event: string, data: string) => void) {
  const reader = body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  for (;;) {
    const { value, done } = await reader.read();
    if (done) {
      return;
    }
    buffer += decoder.decode(value, { stream: true });
    let boundary = buffer.indexOf("\n\n");
    while (boundary >= 0) {
      const chunk = buffer.slice(0, boundary);
      buffer = buffer.slice(boundary + 2);
      dispatchServerSentEvent(chunk, onEvent);
      boundary = buffer.indexOf("\n\n");
    }
  }
}

function dispatchServerSentEvent(chunk: string, onEvent: (event: string, data: string) => void) {
  let event = "message";
  const data: string[] = [];
  for (const line of chunk.split("\n")) {
    if (line.startsWith(":")) {
      continue;
    }
    if (line.startsWith("event:")) {
      event = line.slice("event:".length).trim();
    }
    if (line.startsWith("data:")) {
      data.push(line.slice("data:".length).trimStart());
    }
  }
  onEvent(event, data.join("\n"));
}
