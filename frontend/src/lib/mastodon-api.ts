import { ApiClient } from "@/lib/api";
import type { MastodonAccount, MastodonInstance, MastodonMediaAttachment, MastodonNotification, MastodonRelationship, MastodonSearchResults, MastodonStatus } from "@/types/mastodon";

export type CreateStatusInput = {
  status: string;
  visibility?: "public" | "unlisted" | "private" | "direct";
  sensitive?: boolean;
  spoiler_text?: string;
  in_reply_to_id?: string;
  media_ids?: string[];
};

export function createMastodonApi(accessToken: string) {
  const client = new ApiClient({ accessToken });

  return {
    verifyCredentials() {
      return client.request<MastodonAccount>("/api/v1/accounts/verify_credentials");
    },
    instance() {
      return client.request<MastodonInstance>("/api/v1/instance");
    },
    homeTimeline(options: { limit?: number; maxId?: string } = {}) {
      const params = timelineParams(options);
      return client.request<MastodonStatus[]>(`/api/v1/timelines/home?${params.toString()}`);
    },
    publicTimeline(options: { limit?: number; maxId?: string; local?: boolean; remote?: boolean } = {}) {
      const params = timelineParams(options);
      if (options.local) {
        params.set("local", "true");
      }
      if (options.remote) {
        params.set("remote", "true");
      }
      return client.request<MastodonStatus[]>(`/api/v1/timelines/public?${params.toString()}`);
    },
    createStatus(input: CreateStatusInput) {
      return client.request<MastodonStatus>("/api/v1/statuses", {
        method: "POST",
        body: JSON.stringify(input),
      });
    },
    uploadMedia(file: File, description?: string) {
      const body = new FormData();
      body.set("file", file);
      if (description) {
        body.set("description", description);
      }
      return client.request<MastodonMediaAttachment>("/api/v2/media", {
        method: "POST",
        body,
      });
    },
    media(id: string) {
      return client.request<MastodonMediaAttachment>(`/api/v1/media/${encodeURIComponent(id)}`);
    },
    updateMedia(id: string, description: string) {
      return client.request<MastodonMediaAttachment>(`/api/v1/media/${encodeURIComponent(id)}`, {
        method: "PUT",
        body: JSON.stringify({ description }),
      });
    },
    deleteMedia(id: string) {
      return client.request<void>(`/api/v1/media/${encodeURIComponent(id)}`, {
        method: "DELETE",
      });
    },
    account(id: string) {
      return client.request<MastodonAccount>(`/api/v1/accounts/${encodeURIComponent(id)}`);
    },
    accountStatuses(id: string, options: { limit?: number; maxId?: string } = {}) {
      const params = new URLSearchParams({ limit: String(options.limit ?? 20) });
      if (options.maxId) {
        params.set("max_id", options.maxId);
      }
      return client.request<MastodonStatus[]>(`/api/v1/accounts/${encodeURIComponent(id)}/statuses?${params.toString()}`);
    },
    status(id: string) {
      return client.request<MastodonStatus>(`/api/v1/statuses/${encodeURIComponent(id)}`);
    },
    statusContext(id: string) {
      return client.request<{ ancestors: MastodonStatus[]; descendants: MastodonStatus[] }>(
        `/api/v1/statuses/${encodeURIComponent(id)}/context`,
      );
    },
    deleteStatus(id: string) {
      return client.request<MastodonStatus>(`/api/v1/statuses/${encodeURIComponent(id)}`, {
        method: "DELETE",
      });
    },
    favouriteStatus(id: string) {
      return client.request<MastodonStatus>(`/api/v1/statuses/${encodeURIComponent(id)}/favourite`, { method: "POST" });
    },
    unfavouriteStatus(id: string) {
      return client.request<MastodonStatus>(`/api/v1/statuses/${encodeURIComponent(id)}/unfavourite`, { method: "POST" });
    },
    bookmarkStatus(id: string) {
      return client.request<MastodonStatus>(`/api/v1/statuses/${encodeURIComponent(id)}/bookmark`, { method: "POST" });
    },
    unbookmarkStatus(id: string) {
      return client.request<MastodonStatus>(`/api/v1/statuses/${encodeURIComponent(id)}/unbookmark`, { method: "POST" });
    },
    reblogStatus(id: string) {
      return client.request<MastodonStatus>(`/api/v1/statuses/${encodeURIComponent(id)}/reblog`, { method: "POST" });
    },
    unreblogStatus(id: string) {
      return client.request<MastodonStatus>(`/api/v1/statuses/${encodeURIComponent(id)}/unreblog`, { method: "POST" });
    },
    notifications(limit = 40) {
      return client.request<MastodonNotification[]>(`/api/v1/notifications?limit=${limit}`);
    },
    clearNotifications() {
      return client.request<Record<string, never>>("/api/v1/notifications/clear", { method: "POST" });
    },
    favourites(limit = 40) {
      return client.request<MastodonStatus[]>(`/api/v1/favourites?limit=${limit}`);
    },
    bookmarks(limit = 40) {
      return client.request<MastodonStatus[]>(`/api/v1/bookmarks?limit=${limit}`);
    },
    searchKnownAccounts(query: string, limit = 8) {
      const params = new URLSearchParams({ q: query, limit: String(limit) });
      return client.request<MastodonAccount[]>(`/api/v1/accounts/search?${params.toString()}`);
    },
    searchAccounts(query: string) {
      const params = new URLSearchParams({ q: query, type: "accounts", resolve: "true" });
      return client.request<MastodonSearchResults>(`/api/v2/search?${params.toString()}`);
    },
    relationships(ids: string[]) {
      const params = new URLSearchParams();
      ids.forEach((id) => params.append("id[]", id));
      return client.request<MastodonRelationship[]>(`/api/v1/accounts/relationships?${params.toString()}`);
    },
    followers(id: string) {
      return client.request<MastodonAccount[]>(`/api/v1/accounts/${encodeURIComponent(id)}/followers`);
    },
    following(id: string) {
      return client.request<MastodonAccount[]>(`/api/v1/accounts/${encodeURIComponent(id)}/following`);
    },
    followAccount(id: string) {
      return client.request<MastodonRelationship>(`/api/v1/accounts/${encodeURIComponent(id)}/follow`, {
        method: "POST",
      });
    },
    unfollowAccount(id: string) {
      return client.request<MastodonRelationship>(`/api/v1/accounts/${encodeURIComponent(id)}/unfollow`, {
        method: "POST",
      });
    },
  };
}

function timelineParams(options: { limit?: number; maxId?: string }) {
  const params = new URLSearchParams({ limit: String(options.limit ?? 20) });
  if (options.maxId) {
    params.set("max_id", options.maxId);
  }
  return params;
}
