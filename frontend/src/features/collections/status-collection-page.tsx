import { useCallback, useEffect, useMemo, useState } from "react";

import { useAuth } from "@/app/auth-context";
import { Button } from "@/components/ui/button";
import { EmptyState, FeaturePage, Panel } from "@/features/shared";
import { replaceStatus, runStatusAction } from "@/features/status/status-actions";
import { StatusList, type StatusAction } from "@/features/status/status-list";
import { createMastodonApi } from "@/lib/mastodon-api";
import type { MastodonAccount, MastodonStatus } from "@/types/mastodon";

type StatusCollectionPageProps = {
  type: "bookmarks" | "favourites";
};

export function StatusCollectionPage({ type }: StatusCollectionPageProps) {
  const { session } = useAuth();
  const [currentAccount, setCurrentAccount] = useState<MastodonAccount | null>(null);
  const [statuses, setStatuses] = useState<MastodonStatus[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [actingStatusId, setActingStatusId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const api = useMemo(() => (session?.accessToken ? createMastodonApi(session.accessToken) : null), [session?.accessToken]);

  const title = type === "bookmarks" ? "Bookmarks" : "Favourites";

  const loadStatuses = useCallback(async () => {
    if (!api) return;
    setIsLoading(true);
    setError(null);

    try {
      const [account, nextStatuses] = await Promise.all([
        api.verifyCredentials(),
        type === "bookmarks" ? api.bookmarks() : api.favourites(),
      ]);
      setCurrentAccount(account);
      setStatuses(nextStatuses);
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : `Could not load ${title.toLowerCase()}.`);
    } finally {
      setIsLoading(false);
    }
  }, [api, title, type]);

  useEffect(() => {
    void loadStatuses();
  }, [loadStatuses]);

  async function runAction(action: StatusAction, status: MastodonStatus) {
    if (!api) return;
    setActingStatusId(status.id);
    setError(null);

    try {
      const nextStatus = await runStatusAction(api, action, status);
      if ((type === "bookmarks" && action === "unbookmark") || (type === "favourites" && action === "unfavourite")) {
        setStatuses((current) => current.filter((item) => item.id !== status.id));
      } else {
        setStatuses((current) => replaceStatus(current, nextStatus));
      }
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not update post.");
    } finally {
      setActingStatusId(null);
    }
  }

  return (
    <FeaturePage eyebrow="Library" title={title} description={type === "bookmarks" ? "Posts you saved." : "Posts you favourited."}>
      <Panel title={title}>
        <div className="mb-5 flex justify-end">
          <Button variant="outline" size="sm" onClick={() => void loadStatuses()} disabled={isLoading}>Refresh</Button>
        </div>
        {error ? <p className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive" role="alert">{error}</p> : null}
        {isLoading ? (
          <div className="space-y-3">{[0, 1, 2].map((item) => <div key={item} className="h-24 animate-pulse rounded-md bg-secondary" />)}</div>
        ) : statuses.length === 0 ? (
          <EmptyState title={`No ${title.toLowerCase()}`} description="Nothing to show." />
        ) : (
          <StatusList
            statuses={statuses}
            currentAccountId={currentAccount?.id}
            actingStatusId={actingStatusId}
            emptyTitle={`No ${title.toLowerCase()}`}
            emptyDescription="Nothing to show."
            onAction={runAction}
          />
        )}
      </Panel>
    </FeaturePage>
  );
}
