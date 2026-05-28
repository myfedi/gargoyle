import { useCallback, useEffect, useMemo, useState } from "react";

import { useAuth } from "@/app/auth-context";
import { Button } from "@/components/ui/button";
import { Tabs } from "@/components/ui/tabs";
import { EmptyState, FeaturePage, Panel } from "@/features/shared";
import { replaceStatus, runStatusAction } from "@/features/status/status-actions";
import { StatusList, type StatusAction } from "@/features/status/status-list";
import { createMastodonApi } from "@/lib/mastodon-api";
import type { MastodonAccount, MastodonStatus } from "@/types/mastodon";

type OutboxTab = "posts" | "bookmarks" | "favourites";

const tabs = [
  { value: "posts", label: "Posts" },
  { value: "bookmarks", label: "Bookmarks" },
  { value: "favourites", label: "Favourites" },
] as const;

export function OutboxPage() {
  const { session } = useAuth();
  const [activeTab, setActiveTab] = useState<OutboxTab>("posts");
  const [currentAccount, setCurrentAccount] = useState<MastodonAccount | null>(null);
  const [statuses, setStatuses] = useState<MastodonStatus[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [actingStatusId, setActingStatusId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const api = useMemo(() => (session?.accessToken ? createMastodonApi(session.accessToken) : null), [session?.accessToken]);

  const loadStatuses = useCallback(async () => {
    if (!api) {
      return;
    }

    setIsLoading(true);
    setError(null);

    try {
      const account = await api.verifyCredentials();
      setCurrentAccount(account);
      const nextStatuses =
        activeTab === "posts"
          ? await api.accountStatuses(account.id)
          : activeTab === "bookmarks"
            ? await api.bookmarks()
            : await api.favourites();
      setStatuses(nextStatuses);
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not load posts.");
    } finally {
      setIsLoading(false);
    }
  }, [activeTab, api]);

  useEffect(() => {
    void loadStatuses();
  }, [loadStatuses]);

  async function runAction(action: StatusAction, status: MastodonStatus) {
    if (!api) {
      return;
    }

    setActingStatusId(status.id);
    setError(null);

    try {
      const nextStatus = await runStatusAction(api, action, status);
      setStatuses((current) => replaceStatus(current, nextStatus));
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not update post.");
    } finally {
      setActingStatusId(null);
    }
  }

  async function deleteStatus(status: MastodonStatus) {
    if (!api) {
      return false;
    }

    setError(null);

    try {
      await api.deleteStatus(status.id);
      setStatuses((current) => current.filter((item) => item.id !== status.id));
      return true;
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not delete post.");
      return false;
    }
  }

  return (
    <FeaturePage eyebrow="Activity" title="Outbox" description="Posts, bookmarks, and favourites.">
      <Panel title="Activity">
        <div className="mb-5 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <Tabs value={activeTab} onValueChange={setActiveTab} items={[...tabs]} />
          <Button variant="outline" size="sm" onClick={() => void loadStatuses()} disabled={isLoading}>Refresh</Button>
        </div>

        {error ? (
          <p className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive" role="alert">{error}</p>
        ) : isLoading ? (
          <div className="space-y-3">
            {[0, 1, 2].map((item) => <div key={item} className="h-24 animate-pulse rounded-md bg-secondary" />)}
          </div>
        ) : statuses.length === 0 ? (
          <EmptyState title="Nothing here" description="No posts to show." />
        ) : (
          <StatusList
            statuses={statuses}
            currentAccountId={currentAccount?.id}
            emptyTitle="Nothing here"
            emptyDescription="No posts to show."
            onDelete={deleteStatus}
            actingStatusId={actingStatusId}
            onAction={runAction}
          />
        )}
      </Panel>
    </FeaturePage>
  );
}
