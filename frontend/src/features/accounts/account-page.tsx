import { useCallback, useEffect, useMemo, useState } from "react";

import { useAuth } from "@/app/auth-context";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { DirectMessageForm } from "@/features/direct/direct-message-form";
import { FieldRow, Panel } from "@/features/shared";
import type { ComposeValues } from "@/features/status/compose-form";
import { ReplyComposer } from "@/features/status/reply-composer";
import { replaceStatus, runStatusAction } from "@/features/status/status-actions";
import { StatusList, type StatusAction } from "@/features/status/status-list";
import { createMastodonApi } from "@/lib/mastodon-api";
import { decodeRouteParam } from "@/lib/routes";
import { htmlToPlainText } from "@/lib/text";
import type { MastodonAccount, MastodonRelationship, MastodonStatus } from "@/types/mastodon";

type AccountPageProps = {
  route: string;
};

export function AccountPage({ route }: AccountPageProps) {
  const { session } = useAuth();
  const accountId = decodeRouteParam(route.split("/")[2]);
  const [account, setAccount] = useState<MastodonAccount | null>(null);
  const [statuses, setStatuses] = useState<MastodonStatus[]>([]);
  const [pinnedStatuses, setPinnedStatuses] = useState<MastodonStatus[]>([]);
  const [currentAccount, setCurrentAccount] = useState<MastodonAccount | null>(null);
  const [relationship, setRelationship] = useState<MastodonRelationship | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isLoadingMore, setIsLoadingMore] = useState(false);
  const [isUpdatingFollow, setIsUpdatingFollow] = useState(false);
  const [deletingStatusId, setDeletingStatusId] = useState<string | null>(null);
  const [actingStatusId, setActingStatusId] = useState<string | null>(null);
  const [replyingTo, setReplyingTo] = useState<MastodonStatus | null>(null);
  const [forwardingStatus, setForwardingStatus] = useState<MastodonStatus | null>(null);
  const [isReplying, setIsReplying] = useState(false);
  const [replyError, setReplyError] = useState<string | null>(null);
  const [isAvatarPreviewOpen, setIsAvatarPreviewOpen] = useState(false);
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
      const [nextCurrentAccount, nextAccount, nextStatuses, nextPinnedStatuses] = await Promise.all([
        api.verifyCredentials(),
        api.account(accountId),
        api.accountStatuses(accountId),
        api.accountStatuses(accountId, { pinned: true }),
      ]);
      const [nextRelationship] = nextAccount.id !== nextCurrentAccount.id ? await api.relationships([nextAccount.id]) : [];
      setCurrentAccount(nextCurrentAccount);
      setAccount(nextAccount);
      setRelationship(nextRelationship ?? null);
      setStatuses(nextStatuses);
      setPinnedStatuses(nextPinnedStatuses);
      setReplyingTo(null);
      setReplyError(null);
      setForwardingStatus(null);
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
      if (action === "unpin") {
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

  async function followAccount() {
    if (!api || !account || account.id === currentAccount?.id) {
      return;
    }
    setIsUpdatingFollow(true);
    setError(null);

    try {
      await api.followAccount(account.id);
      const [nextRelationship] = await api.relationships([account.id]);
      setRelationship(nextRelationship ?? null);
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not follow account.");
    } finally {
      setIsUpdatingFollow(false);
    }
  }

  async function unfollowAccount() {
    if (!api || !account || account.id === currentAccount?.id) {
      return;
    }
    setIsUpdatingFollow(true);
    setError(null);

    try {
      await api.unfollowAccount(account.id);
      const [nextRelationship] = await api.relationships([account.id]);
      setRelationship(nextRelationship ?? null);
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not unfollow account.");
    } finally {
      setIsUpdatingFollow(false);
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
    if (!api) {
      return false;
    }

    setError(null);

    try {
      const updated = await api.updateStatus(status.id, {
        status: values.status,
        visibility: values.visibility,
        sensitive: values.sensitive,
        spoiler_text: values.spoilerText,
        media_ids: values.mediaIds,
        activitypub_type: values.objectType,
        poll: values.objectType === "Question" ? { options: values.pollOptions, expires_in: values.pollExpiresIn, multiple: values.pollMultiple } : undefined,
      });
      setStatuses((current) => replaceStatus(current, updated));
      setPinnedStatuses((current) => replaceStatus(current, updated));
      return true;
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not edit post.");
      return false;
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

  async function submitReply(values: ComposeValues) {
    if (!api || !replyingTo) {
      return;
    }

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

  return (
    <section className="space-y-6">
      {error ? (
        <p className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive" role="alert">
          {error}
        </p>
      ) : null}

      {isLoading ? (
        <Panel title="Account">
          <div className="aspect-[598/145] animate-pulse rounded-md bg-secondary" />
        </Panel>
      ) : account ? (
        <Panel title="Profile">
          <div className="mb-4 space-y-4">
            <div className="aspect-[598/145] overflow-hidden rounded-lg border border-border bg-[linear-gradient(135deg,hsl(var(--secondary)),hsl(var(--muted)))]">
              {account.header ? <img className="h-full w-full object-cover" src={account.header} alt="Profile header" /> : null}
            </div>
            <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
              <div className="flex items-center gap-3">
                {account.avatar ? (
                  <button
                    type="button"
                    className="size-16 overflow-hidden rounded-full border border-border object-cover transition-opacity hover:opacity-90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                    onClick={() => setIsAvatarPreviewOpen(true)}
                    aria-label={`View ${(account.display_name || account.username)} avatar`}
                  >
                    <img className="h-full w-full object-cover" src={account.avatar} alt="Profile avatar" />
                  </button>
                ) : null}
                <div>
                  <p className="font-semibold">{account.display_name || account.username}</p>
                  <p className="text-sm text-muted-foreground">@{account.acct}</p>
                </div>
              </div>
              {account.id !== currentAccount?.id ? <FollowButton relationship={relationship} isBusy={isUpdatingFollow} onFollow={followAccount} onUnfollow={unfollowAccount} /> : null}
            </div>
          </div>
          <dl>
            <FieldRow label="Handle" value={`@${account.acct}`} />
            <FieldRow label="Profile" value={account.url ? <a className="text-primary hover:underline" href={account.url} target="_blank" rel="noreferrer">{account.url}</a> : "No URL"} />
            <FieldRow label="Bio" value={account.note ? htmlToPlainText(account.note) : "No bio"} />
          </dl>
        </Panel>
      ) : null}

      {pinnedStatuses.length > 0 ? (
        <Panel title="Pinned posts" className="mx-auto max-w-2xl">
          <StatusList
            statuses={pinnedStatuses}
            currentAccountId={currentAccount?.id}
            emptyTitle="No pinned posts"
            emptyDescription="No posts are pinned."
            deletingStatusId={deletingStatusId}
            actingStatusId={actingStatusId}
            onDelete={deleteStatus}
            onEdit={editStatus}
            onAction={runAction}
            onVotePoll={votePoll}
            onForward={setForwardingStatus}
            onReply={(status) => {
              setReplyingTo(status);
              setReplyError(null);
            }}
            renderAfterStatus={renderReplyComposer}
          />
        </Panel>
      ) : null}

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

      <Panel title="Posts" className="mx-auto max-w-2xl">
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
              onEdit={editStatus}
              onAction={runAction}
              onVotePoll={votePoll}
              onForward={setForwardingStatus}
              onReply={(status) => {
                setReplyingTo(status);
                setReplyError(null);
              }}
              renderAfterStatus={renderReplyComposer}
            />
            {statuses.length > 0 ? (
              <div className="mt-5 flex justify-center">
                <Button variant="outline" onClick={() => void loadMore()} disabled={isLoadingMore}>
                  {isLoadingMore ? "Loading..." : "Load more"}
                </Button>
              </div>
            ) : null}
          </>
        )}
      </Panel>

      <Dialog open={Boolean(forwardingStatus)} onOpenChange={(open) => !open && setForwardingStatus(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Forward by direct message</DialogTitle>
          </DialogHeader>
          {forwardingStatus ? <DirectMessageForm forwardedStatus={forwardingStatus} onSent={() => setForwardingStatus(null)} onCancel={() => setForwardingStatus(null)} /> : null}
        </DialogContent>
      </Dialog>
    </section>
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

type FollowButtonProps = {
  relationship: MastodonRelationship | null;
  isBusy: boolean;
  onFollow: () => void;
  onUnfollow: () => void;
};

function FollowButton({ relationship, isBusy, onFollow, onUnfollow }: FollowButtonProps) {
  const isFollowing = Boolean(relationship?.following);
  const isRequested = Boolean(relationship?.requested);
  if (isFollowing || isRequested) {
    return (
      <Button variant="outline" disabled={isBusy} onClick={onUnfollow}>
        {isBusy ? "Updating..." : isRequested ? "Cancel request" : "Unfollow"}
      </Button>
    );
  }
  return <Button disabled={isBusy} onClick={onFollow}>{isBusy ? "Following..." : "Follow"}</Button>;
}
