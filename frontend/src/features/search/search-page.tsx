import { useCallback, useMemo, useState } from "react";

import { useAuth } from "@/app/auth-context";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { AccountCombobox, normalizeRemoteQuery } from "@/features/accounts/account-combobox";
import { EmptyState, FeaturePage, Panel } from "@/features/shared";
import { createMastodonApi } from "@/lib/mastodon-api";
import { accountHref } from "@/lib/routes";
import { htmlToPlainText } from "@/lib/text";
import type { MastodonAccount, MastodonRelationship } from "@/types/mastodon";

type AccountSearchResult = {
  account: MastodonAccount;
  relationship?: MastodonRelationship;
};

export function SearchPage() {
  const { session } = useAuth();
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<AccountSearchResult[]>([]);
  const [isResolving, setIsResolving] = useState(false);
  const [busyAccountId, setBusyAccountId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const api = useMemo(() => (session?.accessToken ? createMastodonApi(session.accessToken) : null), [session?.accessToken]);

  const searchKnownAccounts = useCallback(async (searchQuery: string) => {
    if (!api) return [];
    return api.searchKnownAccounts(searchQuery);
  }, [api]);

  async function loadResults(accounts: MastodonAccount[]) {
    if (!api) return;
    const ids = accounts.map((account) => account.id);
    const relationships = ids.length > 0 ? await api.relationships(ids) : [];
    const relationshipsById = new Map(relationships.map((relationship) => [relationship.id, relationship]));
    setResults(accounts.map((account) => ({ account, relationship: relationshipsById.get(account.id) })));
  }

  async function resolveAccount(searchQuery: string) {
    if (!api || !searchQuery.trim()) return;
    setIsResolving(true);
    setError(null);

    try {
      const search = await api.searchAccounts(normalizeRemoteQuery(searchQuery));
      await loadResults(search.accounts);
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not look up account.");
    } finally {
      setIsResolving(false);
    }
  }

  async function followAccount(account: MastodonAccount) {
    if (!api) return;
    setBusyAccountId(account.id);
    setError(null);

    try {
      await api.followAccount(account.id);
      const [relationship] = await api.relationships([account.id]);
      if (relationship) updateRelationship(account.id, relationship);
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not follow account.");
    } finally {
      setBusyAccountId(null);
    }
  }

  async function unfollowAccount(account: MastodonAccount) {
    if (!api) return;
    setBusyAccountId(account.id);
    setError(null);

    try {
      await api.unfollowAccount(account.id);
      const [relationship] = await api.relationships([account.id]);
      if (relationship) updateRelationship(account.id, relationship);
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not unfollow account.");
    } finally {
      setBusyAccountId(null);
    }
  }

  function updateRelationship(accountId: string, relationship: MastodonRelationship) {
    setResults((current) => current.map((result) => result.account.id === accountId ? { ...result, relationship } : result));
  }

  return (
    <FeaturePage eyebrow="Search" title="Search" description="Find people by handle or URL.">
      <Panel title="Find people">
        <div className="space-y-5">
          <AccountCombobox
            value={query}
            onValueChange={setQuery}
            searchKnownAccounts={searchKnownAccounts}
            isResolving={isResolving}
            placeholder="Search known accounts or enter @user@example.org"
            onSelect={(account) => void loadResults([account])}
            onResolve={(searchQuery) => void resolveAccount(searchQuery)}
          />
          {error ? <p className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive" role="alert">{error}</p> : null}
          <AccountResults results={results} busyAccountId={busyAccountId} onFollow={followAccount} onUnfollow={unfollowAccount} />
        </div>
      </Panel>
    </FeaturePage>
  );
}

type AccountResultsProps = {
  results: AccountSearchResult[];
  busyAccountId: string | null;
  onFollow: (account: MastodonAccount) => void;
  onUnfollow: (account: MastodonAccount) => void;
};

function AccountResults({ results, busyAccountId, onFollow, onUnfollow }: AccountResultsProps) {
  if (results.length === 0) {
    return <EmptyState title="No results" description="Search for an account to follow." />;
  }

  return (
    <div className="divide-y divide-border">
      {results.map(({ account, relationship }) => {
        const isFollowing = Boolean(relationship?.following);
        const isRequested = Boolean(relationship?.requested);
        const isBusy = busyAccountId === account.id;
        return (
          <article key={account.id} className="flex flex-col gap-4 py-4 first:pt-0 last:pb-0 sm:flex-row sm:items-start sm:justify-between">
            <div className="min-w-0 space-y-1">
              <div className="flex flex-wrap items-center gap-2">
                <a className="text-sm font-semibold hover:underline" href={accountHref(account.id)}>{account.display_name || account.username}</a>
                <p className="text-sm text-muted-foreground">@{account.acct}</p>
                {isRequested ? <Badge variant="secondary">Requested</Badge> : null}
                {isFollowing ? <Badge variant="secondary">Following</Badge> : null}
              </div>
              {account.note ? <p className="max-w-2xl text-sm leading-6 text-muted-foreground">{htmlToPlainText(account.note)}</p> : null}
              {account.url ? <a className="text-sm text-primary hover:underline" href={account.url} target="_blank" rel="noreferrer">{account.url}</a> : null}
            </div>
            {isFollowing || isRequested ? (
              <Button variant="outline" disabled={isBusy} onClick={() => onUnfollow(account)}>{isBusy ? "Updating..." : "Unfollow"}</Button>
            ) : (
              <Button disabled={isBusy} onClick={() => onFollow(account)}>{isBusy ? "Following..." : "Follow"}</Button>
            )}
          </article>
        );
      })}
    </div>
  );
}
