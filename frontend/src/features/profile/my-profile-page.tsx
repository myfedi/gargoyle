import { useCallback, useEffect, useMemo, useState } from "react";

import { useAuth } from "@/app/auth-context";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Tabs } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { EmptyState, FieldRow, FeaturePage, Panel } from "@/features/shared";
import { replaceStatus, runStatusAction } from "@/features/status/status-actions";
import { StatusList, type StatusAction } from "@/features/status/status-list";
import { createMastodonApi } from "@/lib/mastodon-api";
import { accountHref } from "@/lib/routes";
import { htmlToPlainText } from "@/lib/text";
import type { MastodonAccount, MastodonRelationship, MastodonStatus } from "@/types/mastodon";

type ProfileTab = "profile" | "posts" | "pinned" | "bookmarks" | "favourites" | "following" | "followers";

type AccountSearchResult = {
  account: MastodonAccount;
  relationship?: MastodonRelationship;
};

const profileTabs = [
  { value: "profile", label: "Profile" },
  { value: "posts", label: "Posts" },
  { value: "pinned", label: "Pinned" },
  { value: "bookmarks", label: "Bookmarks" },
  { value: "favourites", label: "Favourites" },
  { value: "following", label: "Following" },
  { value: "followers", label: "Followers" },
] as const;

export function MyProfilePage() {
  const { session } = useAuth();
  const [activeTab, setActiveTab] = useState<ProfileTab>("profile");
  const [account, setAccount] = useState<MastodonAccount | null>(null);
  const [statuses, setStatuses] = useState<MastodonStatus[]>([]);
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

  const loadProfile = useCallback(async () => {
    if (!api) return;
    setIsLoading(true);
    setError(null);

    try {
      const nextAccount = await api.verifyCredentials();
      setAccount(nextAccount);
      setProfileForm({ displayName: nextAccount.display_name || "", note: nextAccount.note ? htmlToPlainText(nextAccount.note) : "", avatar: null, header: null });

      if (activeTab === "posts") {
        setStatuses(await api.accountStatuses(nextAccount.id));
      } else if (activeTab === "pinned") {
        setStatuses(await api.accountStatuses(nextAccount.id, { pinned: true }));
      } else if (activeTab === "bookmarks") {
        setStatuses(await api.bookmarks());
      } else if (activeTab === "favourites") {
        setStatuses(await api.favourites());
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


  async function runAction(action: StatusAction, status: MastodonStatus) {
    if (!api) return;
    setActingStatusId(status.id);
    setError(null);

    try {
      const nextStatus = await runStatusAction(api, action, status);
      if ((activeTab === "bookmarks" && action === "unbookmark") || (activeTab === "favourites" && action === "unfavourite") || (activeTab === "pinned" && action === "unpin")) {
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

  async function deleteStatus(status: MastodonStatus) {
    if (!api) return false;
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
    <FeaturePage eyebrow="Profile" title="My profile" description={account ? `@${account.acct}` : "Your account and saved posts."}>
      <Panel title={account?.display_name || account?.username || "Profile"}>
        <Tabs value={activeTab} onValueChange={setActiveTab} items={[...profileTabs]} />

        {error ? <p className="mt-5 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive" role="alert">{error}</p> : null}

        <div className="mt-5">
          {isLoading ? <LoadingRows /> : renderTab()}
        </div>
      </Panel>
    </FeaturePage>
  );

  function renderTab() {
    if (!account) return <EmptyState title="No account" description="Could not load account." />;

    if (activeTab === "profile") {
      if (isEditingProfile) {
        return (
          <form className="space-y-5" onSubmit={(event) => { event.preventDefault(); void saveProfile(); }}>
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
        <div className="space-y-4">
          {account.header ? <img className="h-32 w-full rounded-lg border border-border object-cover" src={account.header} alt="Profile header" /> : null}
          <div className="flex items-start justify-between gap-4">
            <div className="flex items-center gap-3">
              {account.avatar ? <img className="size-16 rounded-full border border-border object-cover" src={account.avatar} alt="Profile avatar" /> : null}
              <div>
                <p className="font-semibold">{account.display_name || account.username}</p>
                <p className="text-sm text-muted-foreground">@{account.acct}</p>
              </div>
            </div>
            <Button variant="outline" onClick={() => setIsEditingProfile(true)}>Edit profile</Button>
          </div>
          <dl>
            <FieldRow label="Handle" value={`@${account.acct}`} />
            <FieldRow label="Profile" value={account.url ? <a className="text-primary hover:underline" href={account.url} target="_blank" rel="noreferrer">{account.url}</a> : "No URL"} />
            <FieldRow label="Bio" value={account.note ? htmlToPlainText(account.note) : "No bio"} />
            <FieldRow label="Posts" value={account.statuses_count ?? 0} />
            <FieldRow label="Following" value={account.following_count ?? 0} />
            <FieldRow label="Followers" value={account.followers_count ?? 0} />
          </dl>
        </div>
      );
    }

    if (activeTab === "posts" || activeTab === "pinned" || activeTab === "bookmarks" || activeTab === "favourites") {
      return (
        <StatusList
          statuses={statuses}
          currentAccountId={account.id}
          actingStatusId={actingStatusId}
          deletingStatusId={deletingStatusId}
          emptyTitle="Nothing here"
          emptyDescription="No posts to show."
          onDelete={activeTab === "posts" || activeTab === "pinned" ? deleteStatus : undefined}
          onAction={runAction}
        />
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

function LoadingRows() {
  return <div className="space-y-3">{[0, 1, 2].map((item) => <div key={item} className="h-20 animate-pulse rounded-md bg-secondary" />)}</div>;
}
