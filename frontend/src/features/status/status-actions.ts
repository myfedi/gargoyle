import type { createMastodonApi } from "@/lib/mastodon-api";
import type { MastodonStatus } from "@/types/mastodon";
import type { StatusAction } from "@/features/status/status-list";

type MastodonApi = ReturnType<typeof createMastodonApi>;

export async function runStatusAction(api: MastodonApi, action: StatusAction, status: MastodonStatus) {
  switch (action) {
    case "bookmark":
      return api.bookmarkStatus(status.id);
    case "unbookmark":
      return api.unbookmarkStatus(status.id);
    case "favourite":
      return api.favouriteStatus(status.id);
    case "unfavourite":
      return api.unfavouriteStatus(status.id);
    case "reblog":
      return api.reblogStatus(status.id);
    case "unreblog":
      return api.unreblogStatus(status.id);
  }
}

export function replaceStatus(statuses: MastodonStatus[], nextStatus: MastodonStatus) {
  return statuses.map((status) => (status.id === nextStatus.id ? nextStatus : status));
}
