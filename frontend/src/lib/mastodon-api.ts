import { ApiClient } from "@/lib/api";
import type { MastodonAccount, MastodonInstance, MastodonRelationship, MastodonSearchResults, MastodonStatus } from "@/types/mastodon";

export type CreateStatusInput = {
  status: string;
  visibility?: "public" | "unlisted" | "private" | "direct";
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
