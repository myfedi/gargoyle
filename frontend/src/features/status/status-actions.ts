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
    case "pin":
      return api.pinStatus(status.id);
    case "unpin":
      return api.unpinStatus(status.id);
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

export function optimisticStatusAction(status: MastodonStatus, action: StatusAction) {
  switch (action) {
    case "favourite":
      return { ...status, favourited: true, favourites_count: status.favourited ? status.favourites_count : status.favourites_count + 1 };
    case "unfavourite":
      return { ...status, favourited: false, favourites_count: status.favourited ? Math.max(0, status.favourites_count - 1) : status.favourites_count };
    case "reblog":
      return { ...status, reblogged: true, reblogs_count: status.reblogged ? status.reblogs_count : status.reblogs_count + 1 };
    case "unreblog":
      return { ...status, reblogged: false, reblogs_count: status.reblogged ? Math.max(0, status.reblogs_count - 1) : status.reblogs_count };
    default:
      return status;
  }
}

export function replaceStatus(statuses: MastodonStatus[], nextStatus: MastodonStatus) {
  return statuses.map((status) => replaceOneStatus(status, nextStatus));
}

export function replaceOneStatus(status: MastodonStatus, nextStatus: MastodonStatus) {
  if (status.id === nextStatus.id) {
    return nextStatus;
  }
  if (status.reblog?.id === nextStatus.id) {
    return { ...status, reblog: nextStatus };
  }
  return status;
}
