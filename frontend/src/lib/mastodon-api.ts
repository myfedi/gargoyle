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
    homeTimeline() {
      return client.request<MastodonStatus[]>("/api/v1/timelines/home");
    },
    publicTimeline() {
      return client.request<MastodonStatus[]>("/api/v1/timelines/public");
    },
    createStatus(input: CreateStatusInput) {
      return client.request<MastodonStatus>("/api/v1/statuses", {
        method: "POST",
        body: JSON.stringify(input),
      });
    },
    searchAccounts(query: string) {
      const params = new URLSearchParams({ q: query });
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
