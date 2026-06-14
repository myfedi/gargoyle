import { getApiBaseUrl, trimTrailingSlash } from "@/lib/config";
import type { AccountUpdate, MastodonNotification, MastodonRelationship } from "@/types/mastodon";

export type NotificationStreamHandlers = {
  onNotification: (notification: MastodonNotification) => void;
  onRelationship?: (relationship: MastodonRelationship) => void;
  onAccountUpdate?: (account: AccountUpdate) => void;
  onError?: (error: Error) => void;
};

export function startNotificationStream(accessToken: string, handlers: NotificationStreamHandlers) {
  let currentController: AbortController | undefined;
  let retryTimeout: number | undefined;
  let reconnectTimeout: number | undefined;
  let stopped = false;
  const watchedRelationships = new Set<string>();

  function streamURL() {
    const url = new URL(`${trimTrailingSlash(getApiBaseUrl())}/api/v1/streaming/user/notification`);
    if (watchedRelationships.size > 0) {
      url.searchParams.set("watch_relationships", [...watchedRelationships].join(","));
    }
    return url.toString();
  }

  function reconnectSoon() {
    if (stopped) {
      return;
    }
    currentController?.abort();
  }

  function watchRelationship(event: Event) {
    const id = (event as CustomEvent<string>).detail;
    if (!id || watchedRelationships.has(id)) {
      return;
    }
    watchedRelationships.add(id);
    reconnectTimeout = globalThis.setTimeout(reconnectSoon, 0);
  }

  async function connect() {
    const requestController = new AbortController();
    currentController = requestController;
    try {
      const response = await fetch(streamURL(), {
        headers: { Accept: "text/event-stream", Authorization: `Bearer ${accessToken}` },
        credentials: "same-origin",
        signal: requestController.signal,
      });
      if (!response.ok || !response.body) {
        throw new Error(`Notification stream failed with status ${response.status}`);
      }
      await readServerSentEvents(response.body, (event, data) => {
        if (!data) {
          return;
        }
        if (event === "notification") {
          handlers.onNotification(JSON.parse(data) as MastodonNotification);
        }
        if (event === "relationship_update") {
          handlers.onRelationship?.(JSON.parse(data) as MastodonRelationship);
        }
        if (event === "account_update") {
          handlers.onAccountUpdate?.(JSON.parse(data) as AccountUpdate);
        }
      });
    } catch (error) {
      if (stopped) {
        return;
      }
      if (!requestController.signal.aborted) {
        handlers.onError?.(error instanceof Error ? error : new Error("Notification stream disconnected."));
      }
    } finally {
      if (currentController === requestController) {
        currentController = undefined;
      }
      if (!stopped) {
        retryTimeout = globalThis.setTimeout(connect, requestController.signal.aborted ? 0 : 3000);
      }
    }
  }

  globalThis.addEventListener("gargoyle:watch-relationship", watchRelationship);
  void connect();

  return () => {
    stopped = true;
    currentController?.abort();
    globalThis.removeEventListener("gargoyle:watch-relationship", watchRelationship);
    if (retryTimeout) {
      globalThis.clearTimeout(retryTimeout);
    }
    if (reconnectTimeout) {
      globalThis.clearTimeout(reconnectTimeout);
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
