import { useCallback, useEffect, useMemo, useState } from "react";

import { useAuth } from "@/app/auth-context";
import { Button } from "@/components/ui/button";
import { FeaturePage, FieldRow, Panel } from "@/features/shared";
import { replaceStatus, runStatusAction } from "@/features/status/status-actions";
import { StatusList, type StatusAction } from "@/features/status/status-list";
import { createMastodonApi } from "@/lib/mastodon-api";
import { decodeRouteParam } from "@/lib/routes";
import { htmlToPlainText } from "@/lib/text";
import type { MastodonAccount, MastodonStatus } from "@/types/mastodon";

type AccountPageProps = {
  route: string;
};

export function AccountPage({ route }: AccountPageProps) {
  const { session } = useAuth();
  const accountId = decodeRouteParam(route.split("/")[2]);
  const [account, setAccount] = useState<MastodonAccount | null>(null);
  const [statuses, setStatuses] = useState<MastodonStatus[]>([]);
  const [currentAccount, setCurrentAccount] = useState<MastodonAccount | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isLoadingMore, setIsLoadingMore] = useState(false);
  const [deletingStatusId, setDeletingStatusId] = useState<string | null>(null);
  const [actingStatusId, setActingStatusId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const api = useMemo(() => (session?.accessToken ? createMastodonApi(session.accessToken) : null), [session?.accessToken]);
  const oldestStatusId = statuses.at(-1)?.id;

  const loadAccount = useCallback(async () => {
    if (!api || !accountId) {
      return;
    }

    setIsLoading(true);
    setError(null);

    try {
      const [nextCurrentAccount, nextAccount, nextStatuses] = await Promise.all([
        api.verifyCredentials(),
        api.account(accountId),
        api.accountStatuses(accountId),
      ]);
      setCurrentAccount(nextCurrentAccount);
      setAccount(nextAccount);
      setStatuses(nextStatuses);
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not load account.");
    } finally {
      setIsLoading(false);
    }
  }, [accountId, api]);

  useEffect(() => {
    void loadAccount();
  }, [loadAccount]);

  async function loadMore() {
    if (!api || !oldestStatusId) {
      return;
    }

    setIsLoadingMore(true);
    setError(null);

    try {
      const nextStatuses = await api.accountStatuses(accountId, { maxId: oldestStatusId });
      setStatuses((current) => [...current, ...nextStatuses]);
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not load more posts.");
    } finally {
      setIsLoadingMore(false);
    }
  }

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

    setDeletingStatusId(status.id);
    setError(null);

    try {
      await api.deleteStatus(status.id);
      setStatuses((current) => current.filter((item) => item.id !== status.id));
      return true;
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not delete post.");
      return false;
    } finally {
      setDeletingStatusId(null);
    }
  }

  return (
    <FeaturePage eyebrow="Account" title={account?.display_name || account?.username || "Account"} description={account ? `@${account.acct}` : ""}>
      {error ? (
        <p className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive" role="alert">
          {error}
        </p>
      ) : null}

      {isLoading ? (
        <Panel title="Account">
          <div className="h-28 animate-pulse rounded-md bg-secondary" />
        </Panel>
      ) : account ? (
        <Panel title="Profile">
          <dl>
            <FieldRow label="Handle" value={`@${account.acct}`} />
            <FieldRow label="Profile" value={account.url ? <a className="text-primary hover:underline" href={account.url} target="_blank" rel="noreferrer">{account.url}</a> : "No URL"} />
            <FieldRow label="Bio" value={account.note ? htmlToPlainText(account.note) : "No bio"} />
          </dl>
        </Panel>
      ) : null}

      <Panel title="Posts">
        {isLoading ? (
          <div className="space-y-3">
            {[0, 1, 2].map((item) => <div key={item} className="h-24 animate-pulse rounded-md bg-secondary" />)}
          </div>
        ) : (
          <>
            <StatusList
              statuses={statuses}
              currentAccountId={currentAccount?.id}
              emptyTitle="No posts"
              emptyDescription="No posts to show."
              deletingStatusId={deletingStatusId}
              actingStatusId={actingStatusId}
              onDelete={deleteStatus}
              onAction={runAction}
            />
            {statuses.length > 0 ? (
              <div className="mt-5">
                <Button variant="outline" onClick={() => void loadMore()} disabled={isLoadingMore}>
                  {isLoadingMore ? "Loading..." : "Load more"}
                </Button>
              </div>
            ) : null}
          </>
        )}
      </Panel>
    </FeaturePage>
  );
}
