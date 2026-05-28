import { useCallback, useEffect, useMemo, useState } from "react";

import { useAuth } from "@/app/auth-context";
import { Button } from "@/components/ui/button";
import { EmptyState, FeaturePage, Panel } from "@/features/shared";
import { runStatusAction } from "@/features/status/status-actions";
import { StatusList, type StatusAction } from "@/features/status/status-list";
import { createMastodonApi } from "@/lib/mastodon-api";
import { accountHref } from "@/lib/routes";
import { formatDateTime } from "@/lib/text";
import type { MastodonAccount, MastodonNotification } from "@/types/mastodon";

export function InboxPage() {
  const { session } = useAuth();
  const [notifications, setNotifications] = useState<MastodonNotification[]>([]);
  const [currentAccount, setCurrentAccount] = useState<MastodonAccount | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [actingStatusId, setActingStatusId] = useState<string | null>(null);
  const [isClearing, setIsClearing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const api = useMemo(() => (session?.accessToken ? createMastodonApi(session.accessToken) : null), [session?.accessToken]);

  const loadNotifications = useCallback(async () => {
    if (!api) {
      return;
    }

    setIsLoading(true);
    setError(null);

    try {
      const [account, items] = await Promise.all([api.verifyCredentials(), api.notifications()]);
      setCurrentAccount(account);
      setNotifications(items);
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not load notifications.");
    } finally {
      setIsLoading(false);
    }
  }, [api]);

  useEffect(() => {
    void loadNotifications();
  }, [loadNotifications]);

  async function runAction(action: StatusAction, status: import("@/types/mastodon").MastodonStatus) {
    if (!api) {
      return;
    }

    setActingStatusId(status.id);
    setError(null);

    try {
      const nextStatus = await runStatusAction(api, action, status);
      setNotifications((current) => current.map((item) => item.status?.id === nextStatus.id ? { ...item, status: nextStatus } : item));
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not update post.");
    } finally {
      setActingStatusId(null);
    }
  }

  async function clearNotifications() {
    if (!api) {
      return;
    }

    setIsClearing(true);
    setError(null);

    try {
      await api.clearNotifications();
      setNotifications([]);
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not clear notifications.");
    } finally {
      setIsClearing(false);
    }
  }

  return (
    <FeaturePage eyebrow="Notifications" title="Inbox" description="Mentions, follows, favourites, boosts, and replies.">
      <Panel title="Notifications">
        <div className="mb-5 flex justify-end gap-2">
          <Button variant="outline" size="sm" onClick={() => void loadNotifications()} disabled={isLoading}>Refresh</Button>
          <Button variant="outline" size="sm" onClick={() => void clearNotifications()} disabled={isClearing || notifications.length === 0}>
            {isClearing ? "Clearing..." : "Clear"}
          </Button>
        </div>

        {error ? (
          <p className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive" role="alert">{error}</p>
        ) : isLoading ? (
          <div className="space-y-3">
            {[0, 1, 2].map((item) => <div key={item} className="h-24 animate-pulse rounded-md bg-secondary" />)}
          </div>
        ) : notifications.length === 0 ? (
          <EmptyState title="No notifications" description="Nothing to show." />
        ) : (
          <div className="divide-y divide-border">
            {notifications.map((notification) => (
              <article key={notification.id} className="py-4 first:pt-0 last:pb-0">
                <div className="mb-3 flex flex-wrap items-center gap-2 text-sm">
                  <a className="font-semibold hover:underline" href={accountHref(notification.account.id)}>
                    {notification.account.display_name || notification.account.username}
                  </a>
                  <span className="text-muted-foreground">{notificationLabel(notification.type)}</span>
                  <time className="ml-auto text-xs text-muted-foreground" dateTime={notification.created_at}>{formatDateTime(notification.created_at)}</time>
                </div>
                {notification.status ? (
                  <StatusList
                    statuses={[notification.status]}
                    currentAccountId={currentAccount?.id}
                    actingStatusId={actingStatusId}
                    onAction={runAction}
                    emptyTitle="No status"
                    emptyDescription="No status attached."
                  />
                ) : null}
              </article>
            ))}
          </div>
        )}
      </Panel>
    </FeaturePage>
  );
}

function notificationLabel(type: string) {
  switch (type) {
    case "follow":
      return "followed you";
    case "mention":
      return "mentioned you";
    case "favourite":
      return "favourited your post";
    case "reblog":
      return "boosted your post";
    case "status":
      return "posted";
    default:
      return type;
  }
}
