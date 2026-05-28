import type React from "react";
import { useEffect, useMemo, useState } from "react";
import { Search } from "lucide-react";

import { useAuth } from "@/app/auth-context";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Tabs } from "@/components/ui/tabs";
import { EmptyState, FeaturePage, Panel } from "@/features/shared";
import { createMastodonApi } from "@/lib/mastodon-api";
import { htmlToPlainText } from "@/lib/text";
import type { MastodonAccount, MastodonRelationship } from "@/types/mastodon";

type AccountSearchResult = {
  account: MastodonAccount;
  relationship?: MastodonRelationship;
};

type FollowTab = "following" | "followers" | "search";

const followTabs = [
  { value: "following", label: "Following" },
  { value: "followers", label: "Followers" },
  { value: "search", label: "Search" },
] as const;

export function FollowsPage() {
  const { session } = useAuth();
  const [activeTab, setActiveTab] = useState<FollowTab>("following");
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<AccountSearchResult[]>([]);
  const [following, setFollowing] = useState<AccountSearchResult[]>([]);
  const [followers, setFollowers] = useState<AccountSearchResult[]>([]);
  const [isSearching, setIsSearching] = useState(false);
  const [isLoadingLists, setIsLoadingLists] = useState(false);
  const [busyAccountId, setBusyAccountId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const api = useMemo(() => (session?.accessToken ? createMastodonApi(session.accessToken) : null), [session?.accessToken]);

  useEffect(() => {
    if (!api) {
      return;
    }

    const currentApi = api;
    let cancelled = false;
    setIsLoadingLists(true);
    setError(null);

    async function loadLists() {
      const account = await currentApi.verifyCredentials();
      const [nextFollowing, nextFollowers] = await Promise.all([currentApi.following(account.id), currentApi.followers(account.id)]);
      const relationshipIds = Array.from(new Set([...nextFollowing, ...nextFollowers].map((account) => account.id)));
      const relationships = relationshipIds.length > 0 ? await currentApi.relationships(relationshipIds) : [];
      const relationshipsById = new Map(relationships.map((relationship) => [relationship.id, relationship]));

      if (!cancelled) {
        setFollowing(nextFollowing.map((account) => ({ account, relationship: relationshipsById.get(account.id) })));
        setFollowers(nextFollowers.map((account) => ({ account, relationship: relationshipsById.get(account.id) })));
      }
    }

    loadLists()
      .catch((caughtError: unknown) => {
        if (!cancelled) {
          setError(caughtError instanceof Error ? caughtError.message : "Could not load follows.");
        }
      })
      .finally(() => {
        if (!cancelled) {
          setIsLoadingLists(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [api]);

  async function searchAccounts(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!api || !query.trim()) {
      return;
    }

    setIsSearching(true);
    setError(null);

    try {
      const search = await api.searchAccounts(normalizeAccountQuery(query));
      const ids = search.accounts.map((account) => account.id);
      const relationships = ids.length > 0 ? await api.relationships(ids) : [];
      const relationshipsById = new Map(relationships.map((relationship) => [relationship.id, relationship]));
      setResults(search.accounts.map((account) => ({ account, relationship: relationshipsById.get(account.id) })));
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not search accounts.");
    } finally {
      setIsSearching(false);
    }
  }

  async function followAccount(account: MastodonAccount) {
    if (!api) {
      return;
    }

    setBusyAccountId(account.id);
    setError(null);

    try {
      await api.followAccount(account.id);
      const [relationship] = await api.relationships([account.id]);
      if (relationship) {
        updateRelationship(account.id, relationship);
        addOrUpdateFollowing(account, relationship);
      }
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not follow account.");
    } finally {
      setBusyAccountId(null);
    }
  }

  async function unfollowAccount(account: MastodonAccount) {
    if (!api) {
      return;
    }

    setBusyAccountId(account.id);
    setError(null);

    try {
      await api.unfollowAccount(account.id);
      const [relationship] = await api.relationships([account.id]);
      if (relationship) {
        updateRelationship(account.id, relationship);
      }
      setFollowing((current) => current.filter((result) => result.account.id !== account.id));
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not unfollow account.");
    } finally {
      setBusyAccountId(null);
    }
  }

  function addOrUpdateFollowing(account: MastodonAccount, relationship: MastodonRelationship) {
    setFollowing((current) => {
      const nextResult = { account, relationship };
      if (current.some((result) => result.account.id === account.id)) {
        return current.map((result) => (result.account.id === account.id ? nextResult : result));
      }
      return [nextResult, ...current];
    });
  }

  function updateRelationship(accountIdToUpdate: string, relationship: MastodonRelationship) {
    const update = (current: AccountSearchResult[]) =>
      current.map((result) => (result.account.id === accountIdToUpdate ? { ...result, relationship } : result));
    setResults(update);
    setFollowing(update);
    setFollowers(update);
  }

  return (
    <FeaturePage eyebrow="People" title="Follows" description="Followers, following, and account search.">
      <Panel title="People">
        <Tabs value={activeTab} onValueChange={setActiveTab} items={[...followTabs]} />

        <div className="mt-5">
          {error ? (
            <p className="mb-4 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive" role="alert">
              {error}
            </p>
          ) : null}

          {activeTab === "search" ? (
            <div className="space-y-5">
              <form className="flex flex-col gap-3 sm:flex-row" onSubmit={(event) => void searchAccounts(event)}>
                <div className="relative flex-1">
                  <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" aria-hidden="true" />
                  <Input
                    value={query}
                    onChange={(event) => setQuery(event.target.value)}
                    className="pl-9"
                    placeholder="@user@example.org or actor URL"
                    aria-label="Search accounts"
                  />
                </div>
                <Button type="submit" disabled={isSearching || !query.trim()}>
                  {isSearching ? "Searching..." : "Search"}
                </Button>
              </form>
              <AccountList
                accounts={results}
                emptyTitle="No results"
                emptyDescription="Search for a remote account."
                busyAccountId={busyAccountId}
                onFollow={followAccount}
                onUnfollow={unfollowAccount}
              />
            </div>
          ) : activeTab === "following" ? (
            isLoadingLists ? (
              <LoadingRows />
            ) : (
              <AccountList
                accounts={following}
                emptyTitle="Not following anyone"
                emptyDescription="Use search to follow an account."
                busyAccountId={busyAccountId}
                onFollow={followAccount}
                onUnfollow={unfollowAccount}
              />
            )
          ) : isLoadingLists ? (
            <LoadingRows />
          ) : (
            <AccountList
              accounts={followers}
              emptyTitle="No followers"
              emptyDescription="Followers will appear here."
              busyAccountId={busyAccountId}
              onFollow={followAccount}
              onUnfollow={unfollowAccount}
            />
          )}
        </div>
      </Panel>
    </FeaturePage>
  );
}

type AccountListProps = {
  accounts: AccountSearchResult[];
  emptyTitle: string;
  emptyDescription: string;
  busyAccountId: string | null;
  onFollow: (account: MastodonAccount) => void;
  onUnfollow: (account: MastodonAccount) => void;
};

function AccountList({ accounts, emptyTitle, emptyDescription, busyAccountId, onFollow, onUnfollow }: AccountListProps) {
  if (accounts.length === 0) {
    return <EmptyState title={emptyTitle} description={emptyDescription} />;
  }

  return (
    <div className="divide-y divide-border">
      {accounts.map(({ account, relationship }) => {
        const isFollowing = Boolean(relationship?.following);
        const isRequested = Boolean(relationship?.requested);
        const isBusy = busyAccountId === account.id;
        return (
          <article key={account.id} className="flex flex-col gap-4 py-4 first:pt-0 last:pb-0 sm:flex-row sm:items-start sm:justify-between">
            <div className="min-w-0 space-y-1">
              <div className="flex flex-wrap items-center gap-2">
                <h2 className="text-sm font-semibold">{account.display_name || account.username}</h2>
                <p className="text-sm text-muted-foreground">@{account.acct}</p>
                {isRequested ? <Badge variant="secondary">Requested</Badge> : null}
                {isFollowing ? <Badge variant="secondary">Following</Badge> : null}
              </div>
              {account.note ? <p className="max-w-2xl text-sm leading-6 text-muted-foreground">{htmlToPlainText(account.note)}</p> : null}
              {account.url ? (
                <a className="text-sm text-primary hover:underline" href={account.url} target="_blank" rel="noreferrer">
                  {account.url}
                </a>
              ) : null}
            </div>
            {isFollowing || isRequested ? (
              <Button variant="outline" disabled={isBusy} onClick={() => onUnfollow(account)}>
                {isBusy ? "Updating..." : "Unfollow"}
              </Button>
            ) : (
              <Button disabled={isBusy} onClick={() => onFollow(account)}>
                {isBusy ? "Following..." : "Follow"}
              </Button>
            )}
          </article>
        );
      })}
    </div>
  );
}

function normalizeAccountQuery(value: string) {
  const query = value.trim();
  if (query.startsWith("http://") || query.startsWith("https://") || query.startsWith("@")) {
    return query;
  }

  if (/^[^@\s]+@[^@\s]+$/.test(query)) {
    return `@${query}`;
  }

  return query;
}

function LoadingRows() {
  return (
    <div className="space-y-3" aria-label="Loading accounts">
      {[0, 1, 2].map((item) => (
        <div key={item} className="h-20 animate-pulse rounded-md bg-secondary" />
      ))}
    </div>
  );
}
