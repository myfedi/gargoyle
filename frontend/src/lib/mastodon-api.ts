import { ApiClient } from "@/lib/api";
import type { MastodonAccount, MastodonInstance, MastodonStatus } from "@/types/mastodon";

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
  };
}
