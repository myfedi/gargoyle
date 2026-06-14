import { useCallback, useEffect, useMemo, useState } from "react";

import { useAuth } from "@/app/auth-context";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { DirectMessageForm } from "@/features/direct/direct-message-form";
import { Tabs } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { EmptyState, Panel } from "@/features/shared";
import { ComposeForm, type ComposeValues } from "@/features/status/compose-form";
import { ReplyComposer } from "@/features/status/reply-composer";
import { optimisticStatusAction, replaceStatus, runStatusAction } from "@/features/status/status-actions";
import { StatusList, type StatusAction } from "@/features/status/status-list";
import { createMastodonApi } from "@/lib/mastodon-api";
import { accountHref } from "@/lib/routes";
import { htmlToPlainText } from "@/lib/text";
import type { MastodonAccount, MastodonAccountField, MastodonRelationship, MastodonStatus } from "@/types/mastodon";

type ProfileTab = "posts" | "following" | "followers" | "requests" | "bookmarks";

type AccountSearchResult = {
  account: MastodonAccount;
  relationship?: MastodonRelationship;
};

type ProfileForm = {
  displayName: string;
  note: string;
  avatar: File | null;
  header: File | null;
  locked: boolean;
  fields: MastodonAccountField[];
};

const profileTabs = [
  { value: "posts", label: "Posts" },
  { value: "following", label: "Following" },
  { value: "followers", label: "Followers" },
  { value: "requests", label: "Follow requests" },
] as const;

export function MyProfilePage() {
  const { session } = useAuth();
  const [activeTab, setActiveTab] = useState<ProfileTab>("posts");
  const [account, setAccount] = useState<MastodonAccount | null>(null);
  const [statuses, setStatuses] = useState<MastodonStatus[]>([]);
  const [pinnedStatuses, setPinnedStatuses] = useState<MastodonStatus[]>([]);
  const [following, setFollowing] = useState<AccountSearchResult[]>([]);
  const [followers, setFollowers] = useState<AccountSearchResult[]>([]);
  const [followRequests, setFollowRequests] = useState<MastodonAccount[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [busyAccountId, setBusyAccountId] = useState<string | null>(null);
  const [actingStatusId, setActingStatusId] = useState<string | null>(null);
  const [deletingStatusId, setDeletingStatusId] = useState<string | null>(null);
  const [isComposeOpen, setIsComposeOpen] = useState(false);
  const [isPosting, setIsPosting] = useState(false);
  const [publishError, setPublishError] = useState<string | null>(null);
  const [replyingTo, setReplyingTo] = useState<MastodonStatus | null>(null);
  const [forwardingStatus, setForwardingStatus] = useState<MastodonStatus | null>(null);
  const [isReplying, setIsReplying] = useState(false);
  const [replyError, setReplyError] = useState<string | null>(null);
  const [isEditingProfile, setIsEditingProfile] = useState(false);
  const [isSavingProfile, setIsSavingProfile] = useState(false);
  const [isAvatarPreviewOpen, setIsAvatarPreviewOpen] = useState(false);
  const [profileForm, setProfileForm] = useState<ProfileForm>({ displayName: "", note: "", avatar: null, header: null, locked: false, fields: [] });
  const [error, setError] = useState<string | null>(null);

  const api = useMemo(() => (session?.accessToken ? createMastodonApi(session.accessToken) : null), [session?.accessToken]);
  const oldestStatusId = statuses.at(-1)?.id;
  const pinnedIDs = new Set(pinnedStatuses.map((status) => status.id));
  const normalStatuses = statuses.filter((status) => !pinnedIDs.has(status.id));

  const searchKnownAccounts = useCallback(async (query: string) => {
    if (!api) return [];
    return api.searchKnownAccounts(query);
  }, [api]);

  const loadProfile = useCallback(async () => {
    if (!api) return;
    setIsLoading(true);
    setError(null);

    try {
      const nextAccount = await api.verifyCredentials();
      setAccount(nextAccount);
      setProfileForm(profileFormFromAccount(nextAccount));

      if (activeTab === "posts") {
        const [nextStatuses, nextPinnedStatuses] = await Promise.all([
          api.accountStatuses(nextAccount.id),
          api.accountStatuses(nextAccount.id, { pinned: true }),
        ]);
        setStatuses(nextStatuses);
        setPinnedStatuses(nextPinnedStatuses);
        setReplyingTo(null);
        setReplyError(null);
        setPublishError(null);
        setForwardingStatus(null);
      } else if (activeTab === "bookmarks") {
        setStatuses(await api.bookmarks());
      } else if (activeTab === "requests") {
        setFollowRequests(await api.followRequests());
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

    const optimisticStatus = optimisticStatusAction(status, action);
    setStatuses((current) => replaceStatus(current, optimisticStatus));
    setPinnedStatuses((current) => replaceStatus(current, optimisticStatus));

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
      setStatuses((current) => replaceStatus(current, status));
      setPinnedStatuses((current) => replaceStatus(current, status));
      setError(caughtError instanceof Error ? caughtError.message : "Could not update post.");
    } finally {
      setActingStatusId(null);
    }
  }

  async function votePoll(status: MastodonStatus, choices: number[]) {
    if (!api) return;
    setError(null);
    try {
      const poll = await api.votePoll(status.poll?.id ?? status.id, choices);
      const applyPoll = (item: MastodonStatus) => item.id === status.id ? { ...item, poll } : item;
      setStatuses((current) => current.map(applyPoll));
      setPinnedStatuses((current) => current.map(applyPoll));
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not vote in poll.");
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

  async function editStatus(status: MastodonStatus, values: ComposeValues) {
    if (!api) return false;
    setError(null);

    try {
      const updated = await api.updateStatus(status.id, { status: values.status, visibility: values.visibility, sensitive: values.sensitive, spoiler_text: values.spoilerText, media_ids: values.mediaIds, activitypub_type: values.objectType, poll: values.objectType === "Question" ? { options: values.pollOptions, expires_in: values.pollExpiresIn, multiple: values.pollMultiple } : undefined });
      setStatuses((current) => replaceStatus(current, updated));
      setPinnedStatuses((current) => replaceStatus(current, updated));
      return true;
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not edit post.");
      return false;
    }
  }

  async function submitPost(values: ComposeValues) {
    if (!api) return;
    setIsPosting(true);
    setPublishError(null);

    try {
      const createdStatus = await api.createStatus({
        status: values.status,
        visibility: values.visibility,
        sensitive: values.sensitive,
        spoiler_text: values.spoilerText,
        media_ids: values.mediaIds,
        activitypub_type: values.objectType,
        poll: values.objectType === "Question" ? { options: values.pollOptions, expires_in: values.pollExpiresIn, multiple: values.pollMultiple } : undefined,
      });
      setStatuses((current) => [createdStatus, ...current.filter((item) => item.id !== createdStatus.id)]);
      setIsComposeOpen(false);
    } catch (caughtError) {
      setPublishError(caughtError instanceof Error ? caughtError.message : "Could not publish post.");
    } finally {
      setIsPosting(false);
    }
  }

  async function submitReply(values: ComposeValues) {
    if (!api || !replyingTo) return;
    setIsReplying(true);
    setReplyError(null);

    try {
      const parentID = replyingTo.id;
      const createdStatus = await api.createStatus({
        status: values.status,
        visibility: values.visibility,
        sensitive: values.sensitive,
        spoiler_text: values.spoilerText,
        media_ids: values.mediaIds,
        activitypub_type: values.objectType,
        poll: values.objectType === "Question" ? { options: values.pollOptions, expires_in: values.pollExpiresIn, multiple: values.pollMultiple } : undefined,
        in_reply_to_id: parentID,
      });
      setReplyingTo(null);
      setStatuses((current) => insertStatusAfter(current, createdStatus, parentID));
    } catch (caughtError) {
      setReplyError(caughtError instanceof Error ? caughtError.message : "Could not post reply.");
    } finally {
      setIsReplying(false);
    }
  }

  function reply(status: MastodonStatus) {
    setReplyingTo(status);
    setReplyError(null);
  }

  function renderReplyComposer(status: MastodonStatus) {
    return replyingTo?.id === status.id ? (
      <ReplyComposer
        status={replyingTo}
        isSubmitting={isReplying}
        error={replyError}
        onCancel={() => setReplyingTo(null)}
        onSubmit={submitReply}
      />
    ) : null;
  }

  async function followAccount(accountToFollow: MastodonAccount) {
    if (!api) return;
    setBusyAccountId(accountToFollow.id);
    setError(null);

    try {
      await api.followAccount(accountToFollow.id);
      globalThis.dispatchEvent(new CustomEvent("gargoyle:watch-relationship", { detail: accountToFollow.id }));
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
      const nextAccount = await api.updateCredentials({ display_name: profileForm.displayName, note: profileForm.note, avatar: profileForm.avatar, header: profileForm.header, locked: profileForm.locked, fields: nonEmptyProfileFields(profileForm.fields) });
      setAccount(nextAccount);
      setStatuses((current) => replaceAccountInStatuses(current, nextAccount));
      setPinnedStatuses((current) => replaceAccountInStatuses(current, nextAccount));
      setProfileForm(profileFormFromAccount(nextAccount));
      setIsEditingProfile(false);
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not update profile.");
    } finally {
      setIsSavingProfile(false);
    }
  }

  function cancelProfileEdit() {
    if (!account) return;
    setProfileForm(profileFormFromAccount(account));
    setIsEditingProfile(false);
  }

  async function decideFollowRequest(account: MastodonAccount, decision: "approve" | "reject") {
    if (!api) return;
    setBusyAccountId(account.id);
    setError(null);

    try {
      if (decision === "approve") {
        await api.authorizeFollowRequest(account.id);
      } else {
        await api.rejectFollowRequest(account.id);
      }
      setFollowRequests((current) => current.filter((item) => item.id !== account.id));
      if (decision === "approve") {
        setFollowers((current) => [{ account }, ...current.filter((item) => item.account.id !== account.id)]);
      }
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not update follow request.");
    } finally {
      setBusyAccountId(null);
    }
  }

  useEffect(() => {
    const handleRelationship = (event: Event) => {
      const relationship = (event as CustomEvent<MastodonRelationship>).detail;
      if (relationship?.id) {
        updateRelationship(relationship.id, relationship);
      }
    };
    globalThis.addEventListener("gargoyle:relationship", handleRelationship);
    return () => globalThis.removeEventListener("gargoyle:relationship", handleRelationship);
  }, []);

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

      <Dialog open={isAvatarPreviewOpen} onOpenChange={setIsAvatarPreviewOpen}>
        <DialogContent className="max-w-xl">
          <DialogHeader>
            <DialogTitle>Profile picture</DialogTitle>
          </DialogHeader>
          {account?.avatar ? (
            <div className="flex justify-center rounded-md bg-background p-2">
              <img className="max-h-[75vh] max-w-full rounded-md object-contain" src={account.avatar} alt={`${account.display_name || account.username} avatar`} />
            </div>
          ) : null}
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(forwardingStatus)} onOpenChange={(open) => !open && setForwardingStatus(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Forward by direct message</DialogTitle>
          </DialogHeader>
          {forwardingStatus ? <DirectMessageForm forwardedStatus={forwardingStatus} onSent={() => setForwardingStatus(null)} onCancel={() => setForwardingStatus(null)} /> : null}
        </DialogContent>
      </Dialog>

      <Panel title={activeTab === "bookmarks" ? "Bookmarks" : activeTab === "requests" ? "Follow requests" : "Activity"} className="mx-auto max-w-2xl">
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
              <label className="text-sm font-medium" htmlFor="profile-header">Header (598:145)</label>
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
          <div className="space-y-3 rounded-md border border-border bg-background p-3">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div>
                <p className="text-sm font-medium">Profile fields</p>
                <p className="text-xs text-muted-foreground">Add up to four label and value pairs, such as pronouns, website, or location.</p>
              </div>
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={isSavingProfile || profileForm.fields.length >= 4}
                onClick={() => setProfileForm((current) => ({ ...current, fields: [...current.fields, { name: "", value: "" }] }))}
              >
                Add field
              </Button>
            </div>
            {profileForm.fields.length > 0 ? (
              <div className="space-y-3">
                {profileForm.fields.map((field, index) => (
                  <div key={index} className="grid gap-2 sm:grid-cols-[minmax(0,1fr)_minmax(0,2fr)_auto]">
                    <Input
                      aria-label={`Profile field ${index + 1} label`}
                      maxLength={255}
                      placeholder="Label"
                      value={field.name}
                      disabled={isSavingProfile}
                      onChange={(event) => setProfileForm((current) => ({ ...current, fields: updateProfileField(current.fields, index, { name: event.target.value }) }))}
                    />
                    <Input
                      aria-label={`Profile field ${index + 1} value`}
                      maxLength={2047}
                      placeholder="Value"
                      value={field.value}
                      disabled={isSavingProfile}
                      onChange={(event) => setProfileForm((current) => ({ ...current, fields: updateProfileField(current.fields, index, { value: event.target.value }) }))}
                    />
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      disabled={isSavingProfile}
                      onClick={() => setProfileForm((current) => ({ ...current, fields: removeProfileField(current.fields, index) }))}
                    >
                      Remove
                    </Button>
                  </div>
                ))}
              </div>
            ) : null}
          </div>
          <label className="flex items-start gap-3 rounded-md border border-border bg-background p-3 text-sm" htmlFor="profile-locked">
            <input
              id="profile-locked"
              className="mt-1 size-4 accent-primary"
              type="checkbox"
              checked={profileForm.locked}
              disabled={isSavingProfile}
              onChange={(event) => setProfileForm((current) => ({ ...current, locked: event.target.checked }))}
            />
            <span>
              <span className="block font-medium">Require follow requests</span>
              <span className="block text-muted-foreground">New followers stay pending until you approve them.</span>
            </span>
          </label>
          <div className="flex flex-wrap gap-2">
            <Button type="submit" disabled={isSavingProfile}>{isSavingProfile ? "Saving..." : "Save profile"}</Button>
            <Button type="button" variant="outline" disabled={isSavingProfile} onClick={cancelProfileEdit}>Cancel</Button>
          </div>
        </form>
      );
    }

    return (
      <div>
        <div className="relative aspect-[598/145] bg-[linear-gradient(135deg,hsl(var(--secondary)),hsl(var(--muted)))]">
          {currentAccount.header ? <img className="h-full w-full object-cover" src={currentAccount.header} alt="Profile header" /> : null}
          <div className="absolute bottom-0 left-5 size-28 translate-y-1/2 overflow-hidden rounded-full border-4 border-card bg-background shadow-sm sm:size-32">
            {currentAccount.avatar ? (
              <button
                type="button"
                className="h-full w-full transition-opacity hover:opacity-90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                onClick={() => setIsAvatarPreviewOpen(true)}
                aria-label={`View ${(currentAccount.display_name || currentAccount.username)} avatar`}
              >
                <img className="h-full w-full object-cover" src={currentAccount.avatar} alt="Profile avatar" />
              </button>
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
          {currentAccount.fields && currentAccount.fields.length > 0 ? (
            <dl className="mt-5 grid gap-2 sm:grid-cols-2">
              {currentAccount.fields.map((field, index) => (
                <div key={`${field.name}-${index}`} className="rounded-md border border-border bg-background px-3 py-2">
                  <dt className="text-xs font-medium uppercase tracking-wide text-muted-foreground">{field.name}</dt>
                  <dd className="mt-1 break-words text-sm">{htmlToPlainText(field.value)}</dd>
                </div>
              ))}
            </dl>
          ) : null}
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
          onDelete={deleteStatus}
          onEdit={editStatus}
          onAction={runAction}
          onVotePoll={votePoll}
          onForward={setForwardingStatus}
          onReply={reply}
          renderAfterStatus={renderReplyComposer}
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
                onEdit={editStatus}
                onAction={runAction}
                onVotePoll={votePoll}
                onForward={setForwardingStatus}
                onReply={reply}
                renderAfterStatus={renderReplyComposer}
              />
            </section>
          ) : null}
          <section>
            <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
              <h2 className="text-sm font-semibold">Posts</h2>
              <Button size="sm" variant={isComposeOpen ? "outline" : "default"} onClick={() => { setIsComposeOpen((current) => !current); setPublishError(null); }}>
                {isComposeOpen ? "Close composer" : "New post"}
              </Button>
            </div>
            {isComposeOpen ? (
              <div className="mb-5 rounded-lg border border-border bg-background p-4">
                <ComposeForm
                  submitLabel="Publish"
                  submittingLabel="Publishing..."
                  placeholder="Write a post"
                  isSubmitting={isPosting}
                  error={publishError}
                  onSubmit={submitPost}
                  onUploadMedia={api?.uploadMedia}
                  onDeleteMedia={api?.deleteMedia}
                  onUpdateMedia={api?.updateMedia}
                  searchKnownAccounts={searchKnownAccounts}
                />
              </div>
            ) : null}
            <StatusList
              statuses={normalStatuses}
              currentAccountId={account.id}
              actingStatusId={actingStatusId}
              deletingStatusId={deletingStatusId}
              emptyTitle="No posts"
              emptyDescription="No posts to show."
              onDelete={deleteStatus}
              onEdit={editStatus}
              onAction={runAction}
              onVotePoll={votePoll}
              onForward={setForwardingStatus}
              onReply={reply}
              renderAfterStatus={renderReplyComposer}
            />
            {statuses.length > 0 ? (
              <div className="mt-5 flex justify-center">
                <Button variant="outline" onClick={() => void loadMore()} disabled={isLoading}>{isLoading ? "Loading..." : "Load more"}</Button>
              </div>
            ) : null}
          </section>
        </div>
      );
    }

    if (activeTab === "requests") {
      return <FollowRequestList accounts={followRequests} busyAccountId={busyAccountId} onDecision={decideFollowRequest} />;
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

type FollowRequestListProps = {
  accounts: MastodonAccount[];
  busyAccountId: string | null;
  onDecision: (account: MastodonAccount, decision: "approve" | "reject") => void;
};

function FollowRequestList({ accounts, busyAccountId, onDecision }: FollowRequestListProps) {
  if (accounts.length === 0) return <EmptyState title="No follow requests" description="Pending follow requests will appear here." />;

  return (
    <div className="divide-y divide-border">
      {accounts.map((account) => {
        const isBusy = busyAccountId === account.id;
        return (
          <article key={account.id} className="flex flex-col gap-4 py-4 first:pt-0 last:pb-0 sm:flex-row sm:items-start sm:justify-between">
            <div className="flex min-w-0 gap-3">
              {account.avatar ? <img className="size-10 rounded-full border border-border object-cover" src={account.avatar} alt="" aria-hidden="true" /> : null}
              <div className="min-w-0 space-y-1">
                <div className="flex flex-wrap items-center gap-2">
                  <a className="text-sm font-semibold hover:underline" href={accountHref(account.id)}>{account.display_name || account.username}</a>
                  <p className="text-sm text-muted-foreground">@{account.acct}</p>
                  <Badge variant="secondary">Pending</Badge>
                </div>
                {account.note ? <p className="max-w-2xl text-sm leading-6 text-muted-foreground">{htmlToPlainText(account.note)}</p> : null}
              </div>
            </div>
            <div className="flex flex-wrap gap-2">
              <Button disabled={isBusy} onClick={() => onDecision(account, "approve")}>{isBusy ? "Updating..." : "Approve"}</Button>
              <Button variant="outline" disabled={isBusy} onClick={() => onDecision(account, "reject")}>Reject</Button>
            </div>
          </article>
        );
      })}
    </div>
  );
}

function insertStatusAfter(statuses: MastodonStatus[], status: MastodonStatus, parentID: string) {
  if (statuses.some((item) => item.id === status.id)) {
    return statuses;
  }
  const parentIndex = statuses.findIndex((item) => (item.reblog ?? item).id === parentID);
  if (parentIndex === -1) {
    return [status, ...statuses];
  }
  return [...statuses.slice(0, parentIndex + 1), status, ...statuses.slice(parentIndex + 1)];
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

function profileFormFromAccount(account: MastodonAccount): ProfileForm {
  return {
    displayName: account.display_name || "",
    note: account.note ? htmlToPlainText(account.note) : "",
    avatar: null,
    header: null,
    locked: Boolean(account.locked),
    fields: (account.fields ?? []).slice(0, 4).map((field) => ({ name: field.name ?? "", value: field.value ? htmlToPlainText(field.value) : "" })),
  };
}

function updateProfileField(fields: MastodonAccountField[], index: number, patch: Partial<MastodonAccountField>): MastodonAccountField[] {
  return fields.map((field, currentIndex) => currentIndex === index ? { ...field, ...patch } : field);
}

function removeProfileField(fields: MastodonAccountField[], index: number): MastodonAccountField[] {
  return fields.filter((_, currentIndex) => currentIndex !== index);
}

function nonEmptyProfileFields(fields: MastodonAccountField[]): MastodonAccountField[] {
  return fields.map((field) => ({ name: field.name.trim(), value: field.value.trim() })).filter((field) => field.name || field.value);
}

function ProfileSkeleton() {
  return <div className="aspect-[598/145] animate-pulse bg-secondary" />;
}

function LoadingRows() {
  return <div className="space-y-3">{[0, 1, 2].map((item) => <div key={item} className="h-20 animate-pulse rounded-md bg-secondary" />)}</div>;
}
