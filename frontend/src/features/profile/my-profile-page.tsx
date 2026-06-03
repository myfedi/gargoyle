import { useCallback, useEffect, useMemo, useState } from "react";

import { useAuth } from "@/app/auth-context";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Tabs } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { EmptyState, Panel } from "@/features/shared";
import { replaceStatus, runStatusAction } from "@/features/status/status-actions";
import { StatusList, type StatusAction } from "@/features/status/status-list";
import { createMastodonApi } from "@/lib/mastodon-api";
import { accountHref } from "@/lib/routes";
import { htmlToPlainText } from "@/lib/text";
import type { MastodonAccount, MastodonRelationship, MastodonStatus } from "@/types/mastodon";

type ProfileTab = "posts" | "following" | "followers" | "bookmarks";

type AccountSearchResult = {
  account: MastodonAccount;
  relationship?: MastodonRelationship;
};

const profileTabs = [
  { value: "posts", label: "Posts" },
  { value: "following", label: "Following" },
  { value: "followers", label: "Followers" },
] as const;

export function MyProfilePage() {
  const { session } = useAuth();
  const [activeTab, setActiveTab] = useState<ProfileTab>("posts");
  const [account, setAccount] = useState<MastodonAccount | null>(null);
  const [statuses, setStatuses] = useState<MastodonStatus[]>([]);
  const [pinnedStatuses, setPinnedStatuses] = useState<MastodonStatus[]>([]);
  const [following, setFollowing] = useState<AccountSearchResult[]>([]);
  const [followers, setFollowers] = useState<AccountSearchResult[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [busyAccountId, setBusyAccountId] = useState<string | null>(null);
  const [actingStatusId, setActingStatusId] = useState<string | null>(null);
  const [deletingStatusId, setDeletingStatusId] = useState<string | null>(null);
  const [isEditingProfile, setIsEditingProfile] = useState(false);
  const [isSavingProfile, setIsSavingProfile] = useState(false);
  const [profileForm, setProfileForm] = useState<{ displayName: string; note: string; avatar: File | null; header: File | null }>({ displayName: "", note: "", avatar: null, header: null });
  const [error, setError] = useState<string | null>(null);

  const api = useMemo(() => (session?.accessToken ? createMastodonApi(session.accessToken) : null), [session?.accessToken]);
  const oldestStatusId = statuses.at(-1)?.id;
  const pinnedIDs = new Set(pinnedStatuses.map((status) => status.id));
  const normalStatuses = statuses.filter((status) => !pinnedIDs.has(status.id));

  const loadProfile = useCallback(async () => {
    if (!api) return;
    setIsLoading(true);
    setError(null);

    try {
      const nextAccount = await api.verifyCredentials();
      setAccount(nextAccount);
      setProfileForm({ displayName: nextAccount.display_name || "", note: nextAccount.note ? htmlToPlainText(nextAccount.note) : "", avatar: null, header: null });

      if (activeTab === "posts") {
        const [nextStatuses, nextPinnedStatuses] = await Promise.all([
          api.accountStatuses(nextAccount.id),
          api.accountStatuses(nextAccount.id, { pinned: true }),
        ]);
        setStatuses(nextStatuses);
        setPinnedStatuses(nextPinnedStatuses);
      } else if (activeTab === "bookmarks") {
        setStatuses(await api.bookmarks());
      } else if (activeTab === "following" || activeTab === "followers") {
        const accounts = activeTab === "following" ? await api.following(nextAccount.id) : await api.followers(nextAccount.id);
        const ids = accounts.map((item) => item.id);
        const relationships = ids.length > 0 ? await api.relationships(ids) : [];
        const byId = new Map(relationships.map((relationship) => [relationship.id, relationship]));
        const nextResults = accounts.map((item) => ({ account: item, relationship: byId.get(item.id) }));
        if (activeTab === "following") setFollowing(nextResults);
        else setFollowers(nextResults);
      }
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not load profile.");
    } finally {
      setIsLoading(false);
    }
  }, [activeTab, api]);

  useEffect(() => {
    void loadProfile();
  }, [loadProfile]);

  async function loadMore() {
    if (!api || !account || !oldestStatusId) return;
    setIsLoading(true);
    setError(null);

    try {
      const nextStatuses = await api.accountStatuses(account.id, { maxId: oldestStatusId });
      setStatuses((current) => [...current, ...nextStatuses]);
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not load more posts.");
    } finally {
      setIsLoading(false);
    }
  }

  async function runAction(action: StatusAction, status: MastodonStatus) {
    if (!api) return;
    setActingStatusId(status.id);
    setError(null);

    try {
      const nextStatus = await runStatusAction(api, action, status);
      setStatuses((current) => replaceStatus(current, nextStatus));
      if (activeTab === "bookmarks" && action === "unbookmark") {
        setStatuses((current) => current.filter((item) => item.id !== status.id));
      } else if (action === "unpin") {
        setPinnedStatuses((current) => current.filter((item) => item.id !== status.id));
      } else if (action === "pin") {
        setPinnedStatuses((current) => [nextStatus, ...current.filter((item) => item.id !== nextStatus.id)]);
      } else {
        setPinnedStatuses((current) => replaceStatus(current, nextStatus));
      }
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not update post.");
    } finally {
      setActingStatusId(null);
    }
  }

  async function deleteStatus(status: MastodonStatus) {
    if (!api) return false;
    setDeletingStatusId(status.id);
    setError(null);

    try {
      await api.deleteStatus(status.id);
      setStatuses((current) => current.filter((item) => item.id !== status.id));
      setPinnedStatuses((current) => current.filter((item) => item.id !== status.id));
      return true;
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not delete post.");
      return false;
    } finally {
      setDeletingStatusId(null);
    }
  }

  async function followAccount(accountToFollow: MastodonAccount) {
    if (!api) return;
    setBusyAccountId(accountToFollow.id);
    setError(null);

    try {
      await api.followAccount(accountToFollow.id);
      const [relationship] = await api.relationships([accountToFollow.id]);
      if (relationship) updateRelationship(accountToFollow.id, relationship);
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not follow account.");
    } finally {
      setBusyAccountId(null);
    }
  }

  async function unfollowAccount(accountToUnfollow: MastodonAccount) {
    if (!api) return;
    setBusyAccountId(accountToUnfollow.id);
    setError(null);

    try {
      await api.unfollowAccount(accountToUnfollow.id);
      const [relationship] = await api.relationships([accountToUnfollow.id]);
      if (relationship) updateRelationship(accountToUnfollow.id, relationship);
      setFollowing((current) => current.filter((item) => item.account.id !== accountToUnfollow.id));
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not unfollow account.");
    } finally {
      setBusyAccountId(null);
    }
  }

  async function saveProfile() {
    if (!api) return;
    setIsSavingProfile(true);
    setError(null);

    try {
      const nextAccount = await api.updateCredentials({ display_name: profileForm.displayName, note: profileForm.note, avatar: profileForm.avatar, header: profileForm.header });
      setAccount(nextAccount);
      setStatuses((current) => replaceAccountInStatuses(current, nextAccount));
      setPinnedStatuses((current) => replaceAccountInStatuses(current, nextAccount));
      setProfileForm({ displayName: nextAccount.display_name || "", note: nextAccount.note ? htmlToPlainText(nextAccount.note) : "", avatar: null, header: null });
      setIsEditingProfile(false);
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not update profile.");
    } finally {
      setIsSavingProfile(false);
    }
  }

  function cancelProfileEdit() {
    if (!account) return;
    setProfileForm({ displayName: account.display_name || "", note: account.note ? htmlToPlainText(account.note) : "", avatar: null, header: null });
    setIsEditingProfile(false);
  }

  function updateRelationship(accountId: string, relationship: MastodonRelationship) {
    const update = (current: AccountSearchResult[]) => current.map((item) => item.account.id === accountId ? { ...item, relationship } : item);
    setFollowing(update);
    setFollowers(update);
  }

  return (
    <section className="space-y-6">
      <div className="overflow-hidden rounded-lg border border-border bg-card shadow-sm">
        {isLoading && !account ? <ProfileSkeleton /> : account ? renderProfileDetails(account) : <EmptyState title="No account" description="Could not load account." />}
      </div>

      <Panel title={activeTab === "bookmarks" ? "Bookmarks" : "Activity"}>
        <Tabs value={activeTab} onValueChange={setActiveTab} items={[...profileTabs]} />

        {error ? <p className="mt-5 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive" role="alert">{error}</p> : null}

        <div className="mt-5">
          {isLoading && !account ? <LoadingRows /> : renderTab()}
        </div>
      </Panel>
    </section>
  );

  function renderProfileDetails(currentAccount: MastodonAccount) {
    if (isEditingProfile) {
      return (
        <form className="space-y-5 p-5" onSubmit={(event) => { event.preventDefault(); void saveProfile(); }}>
          <div className="grid gap-2">
            <label className="text-sm font-medium" htmlFor="profile-display-name">Display name</label>
            <Input
              id="profile-display-name"
              maxLength={120}
              value={profileForm.displayName}
              onChange={(event) => setProfileForm((current) => ({ ...current, displayName: event.target.value }))}
              disabled={isSavingProfile}
            />
          </div>
          <div className="grid gap-2 sm:grid-cols-2">
            <div className="grid gap-2">
              <label className="text-sm font-medium" htmlFor="profile-avatar">Avatar</label>
              <Input id="profile-avatar" type="file" accept="image/png,image/jpeg,image/gif,image/webp" disabled={isSavingProfile} onChange={(event) => setProfileForm((current) => ({ ...current, avatar: event.target.files?.[0] ?? null }))} />
            </div>
            <div className="grid gap-2">
              <label className="text-sm font-medium" htmlFor="profile-header">Header</label>
              <Input id="profile-header" type="file" accept="image/png,image/jpeg,image/gif,image/webp" disabled={isSavingProfile} onChange={(event) => setProfileForm((current) => ({ ...current, header: event.target.files?.[0] ?? null }))} />
            </div>
          </div>
          <div className="grid gap-2">
            <label className="text-sm font-medium" htmlFor="profile-note">Bio</label>
            <Textarea
              id="profile-note"
              maxLength={5000}
              value={profileForm.note}
              onChange={(event) => setProfileForm((current) => ({ ...current, note: event.target.value }))}
              disabled={isSavingProfile}
            />
            <p className="text-xs text-muted-foreground">Your updated profile is federated to followers after it is saved.</p>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button type="submit" disabled={isSavingProfile}>{isSavingProfile ? "Saving..." : "Save profile"}</Button>
            <Button type="button" variant="outline" disabled={isSavingProfile} onClick={cancelProfileEdit}>Cancel</Button>
          </div>
        </form>
      );
    }

    return (
      <div>
        <div className="relative h-48 bg-[linear-gradient(135deg,hsl(var(--secondary)),hsl(var(--muted)))] sm:h-60">
          {currentAccount.header ? <img className="h-full w-full object-cover" src={currentAccount.header} alt="Profile header" /> : null}
          <div className="absolute bottom-0 left-5 size-28 translate-y-1/2 overflow-hidden rounded-full border-4 border-card bg-background shadow-sm sm:size-32">
            {currentAccount.avatar ? (
              <img className="h-full w-full object-cover" src={currentAccount.avatar} alt="Profile avatar" />
            ) : (
              <div className="flex h-full w-full items-center justify-center bg-secondary text-3xl font-semibold text-secondary-foreground">{profileInitials(currentAccount)}</div>
            )}
          </div>
        </div>
        <div className="px-5 pb-5 pt-16 sm:pl-40 sm:pt-5">
          <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <h1 className="text-2xl font-semibold tracking-tight">{currentAccount.display_name || currentAccount.username}</h1>
              <p className="text-sm text-muted-foreground">@{currentAccount.acct}</p>
            </div>
            <div className="flex flex-wrap gap-2">
              <Button variant="outline" onClick={() => setActiveTab("bookmarks")}>Bookmarks</Button>
              <Button variant="outline" onClick={() => setIsEditingProfile(true)}>Edit profile</Button>
            </div>
          </div>

          {currentAccount.note ? <p className="mt-5 max-w-3xl text-sm leading-6 text-muted-foreground">{htmlToPlainText(currentAccount.note)}</p> : null}
        </div>
      </div>
    );
  }

  function renderTab() {
    if (!account) return <EmptyState title="No account" description="Could not load account." />;

    if (activeTab === "bookmarks") {
      return (
        <StatusList
          statuses={statuses}
          currentAccountId={account.id}
          actingStatusId={actingStatusId}
          deletingStatusId={deletingStatusId}
          emptyTitle="No bookmarks"
          emptyDescription="Bookmarked posts will appear here."
          onAction={runAction}
        />
      );
    }

    if (activeTab === "posts") {
      return (
        <div className="space-y-7">
          {pinnedStatuses.length > 0 ? (
            <section>
              <h2 className="mb-3 text-sm font-semibold">Pinned posts</h2>
              <StatusList
                statuses={pinnedStatuses}
                currentAccountId={account.id}
                actingStatusId={actingStatusId}
                deletingStatusId={deletingStatusId}
                emptyTitle="No pinned posts"
                emptyDescription="No posts are pinned."
                onDelete={deleteStatus}
                onAction={runAction}
              />
            </section>
          ) : null}
          <section>
            <h2 className="mb-3 text-sm font-semibold">Posts</h2>
            <StatusList
              statuses={normalStatuses}
              currentAccountId={account.id}
              actingStatusId={actingStatusId}
              deletingStatusId={deletingStatusId}
              emptyTitle="No posts"
              emptyDescription="No posts to show."
              onDelete={deleteStatus}
              onAction={runAction}
            />
            {statuses.length > 0 ? (
              <div className="mt-5">
                <Button variant="outline" onClick={() => void loadMore()} disabled={isLoading}>{isLoading ? "Loading..." : "Load more"}</Button>
              </div>
            ) : null}
          </section>
        </div>
      );
    }

    const accounts = activeTab === "following" ? following : followers;
    return <AccountList accounts={accounts} busyAccountId={busyAccountId} onFollow={followAccount} onUnfollow={unfollowAccount} emptyTitle={activeTab === "following" ? "Not following anyone" : "No followers"} />;
  }
}

type AccountListProps = {
  accounts: AccountSearchResult[];
  busyAccountId: string | null;
  emptyTitle: string;
  onFollow: (account: MastodonAccount) => void;
  onUnfollow: (account: MastodonAccount) => void;
};

function AccountList({ accounts, busyAccountId, emptyTitle, onFollow, onUnfollow }: AccountListProps) {
  if (accounts.length === 0) return <EmptyState title={emptyTitle} description="Nothing to show." />;

  return (
    <div className="divide-y divide-border">
      {accounts.map(({ account, relationship }) => {
        const isFollowing = Boolean(relationship?.following);
        const isRequested = Boolean(relationship?.requested);
        const isBusy = busyAccountId === account.id;
        return (
          <article key={account.id} className="flex flex-col gap-4 py-4 first:pt-0 last:pb-0 sm:flex-row sm:items-start sm:justify-between">
            <div className="flex min-w-0 gap-3">
              {account.avatar ? <img className="size-10 rounded-full border border-border object-cover" src={account.avatar} alt="" aria-hidden="true" /> : null}
              <div className="min-w-0 space-y-1">
                <div className="flex flex-wrap items-center gap-2">
                  <a className="text-sm font-semibold hover:underline" href={accountHref(account.id)}>{account.display_name || account.username}</a>
                  <p className="text-sm text-muted-foreground">@{account.acct}</p>
                  {isRequested ? <Badge variant="secondary">Requested</Badge> : null}
                  {isFollowing ? <Badge variant="secondary">Following</Badge> : null}
                </div>
                {account.note ? <p className="max-w-2xl text-sm leading-6 text-muted-foreground">{htmlToPlainText(account.note)}</p> : null}
              </div>
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

function profileInitials(account: MastodonAccount) {
  const source = account.display_name || account.username || account.acct || "?";
  return source.trim().slice(0, 2).toUpperCase();
}

function replaceAccountInStatuses(statuses: MastodonStatus[], account: MastodonAccount): MastodonStatus[] {
  return statuses.map((status) => {
    const nextStatus = status.account.id === account.id ? { ...status, account } : status;
    if (nextStatus.reblog) {
      return { ...nextStatus, reblog: replaceAccountInStatuses([nextStatus.reblog], account)[0] };
    }
    return nextStatus;
  });
}

function ProfileSkeleton() {
  return <div className="h-72 animate-pulse bg-secondary" />;
}

function LoadingRows() {
  return <div className="space-y-3">{[0, 1, 2].map((item) => <div key={item} className="h-20 animate-pulse rounded-md bg-secondary" />)}</div>;
}
