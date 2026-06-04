import { Check, MoreHorizontal } from "lucide-react";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { EmptyState } from "@/features/shared";
import { ComposeForm, type ComposeValues } from "@/features/status/compose-form";
import { StatusBody } from "@/features/status/status-body";
import { accountHref, statusHref } from "@/lib/routes";
import { formatDateTime, htmlToPlainText } from "@/lib/text";
import type { ActivityPubObjectType, MastodonMediaAttachment, MastodonStatus } from "@/types/mastodon";

export type StatusAction = "bookmark" | "unbookmark" | "pin" | "unpin" | "favourite" | "unfavourite" | "reblog" | "unreblog";

type StatusListProps = {
  statuses: MastodonStatus[];
  currentAccountId?: string;
  emptyTitle: string;
  emptyDescription: string;
  deletingStatusId?: string | null;
  actingStatusId?: string | null;
  onDelete?: (status: MastodonStatus) => Promise<boolean> | boolean;
  onEdit?: (status: MastodonStatus, values: ComposeValues) => Promise<boolean> | boolean;
  onReply?: (status: MastodonStatus) => void;
  onForward?: (status: MastodonStatus) => void;
  onAction?: (action: StatusAction, status: MastodonStatus) => Promise<void> | void;
  onVotePoll?: (status: MastodonStatus, choices: number[]) => Promise<void> | void;
};

export function StatusList({
  statuses,
  currentAccountId,
  emptyTitle,
  emptyDescription,
  deletingStatusId,
  actingStatusId,
  onDelete,
  onEdit,
  onReply,
  onForward,
  onAction,
  onVotePoll,
}: StatusListProps) {
  const [statusPendingDeletion, setStatusPendingDeletion] = useState<MastodonStatus | null>(null);
  const [statusBeingEdited, setStatusBeingEdited] = useState<MastodonStatus | null>(null);
  const [isEditingStatus, setIsEditingStatus] = useState(false);
  const [editError, setEditError] = useState<string | null>(null);
  const [mediaPreview, setMediaPreview] = useState<MastodonMediaAttachment | null>(null);

  if (statuses.length === 0) {
    return (
      <div className="mx-auto w-full max-w-2xl">
        <EmptyState title={emptyTitle} description={emptyDescription} />
      </div>
    );
  }

  const isDeleting = Boolean(statusPendingDeletion && deletingStatusId === statusPendingDeletion.id);

  return (
    <>
      <div className="mx-auto w-full max-w-2xl divide-y divide-border">
        {statuses.map((status) => {
          const displayedStatus = status.reblog ?? status;
          const canOwn = Boolean(currentAccountId && displayedStatus.account.id === currentAccountId);
          const canDelete = Boolean(onDelete && canOwn);
          const canEdit = Boolean(onEdit && canOwn);
          const canPin = Boolean(onAction && canOwn && displayedStatus.visibility !== "direct");
          const canReply = Boolean(onReply);
          const canForward = Boolean(onForward);
          const canInteract = Boolean(onAction);
          const isActing = actingStatusId === displayedStatus.id;
          return (
            <article key={status.id} data-status-id={(status.reblog ?? status).id} className="py-4 first:pt-0 last:pb-0">
              {status.reblog ? (
                <p className="mb-2 text-xs text-muted-foreground">
                  {status.account.display_name || status.account.username} boosted
                </p>
              ) : null}
              <div className="flex items-start gap-3">
                {displayedStatus.account.avatar ? <img className="size-10 rounded-full border border-border object-cover" src={displayedStatus.account.avatar} alt="" aria-hidden="true" /> : null}
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
                    <a className="text-sm font-semibold hover:underline" href={accountHref(displayedStatus.account.id)}>
                      {displayedStatus.account.display_name || displayedStatus.account.username}
                    </a>
                    <p className="text-xs text-muted-foreground">@{displayedStatus.account.acct}</p>
                    <StatusTypeBadge status={displayedStatus} />
                    <StatusMeta status={displayedStatus} />
                    <a className="ml-auto text-xs text-muted-foreground hover:underline" href={statusHref(displayedStatus.id)}>
                      <time dateTime={displayedStatus.created_at}>{formatDateTime(displayedStatus.created_at)}</time>
                    </a>
                  </div>
                  <StatusBody html={displayedStatus.content} mentions={displayedStatus.mentions} spoilerText={displayedStatus.spoiler_text}>
                    <StatusPoll status={displayedStatus} onVote={onVotePoll ? (choices) => onVotePoll(displayedStatus, choices) : undefined} />
                    <StatusMedia attachments={displayedStatus.media_attachments ?? []} onPreview={setMediaPreview} />
                  </StatusBody>
                  <StatusStats status={displayedStatus} />
                </div>
                {canDelete || canEdit || canReply || canForward || canInteract ? (
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button variant="ghost" size="icon" aria-label="Post actions" disabled={isActing}>
                        <MoreHorizontal className="size-4" aria-hidden="true" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      {canReply ? <DropdownMenuItem onSelect={() => onReply?.(displayedStatus)}>Reply</DropdownMenuItem> : null}
                      {canEdit ? <DropdownMenuItem onSelect={() => { setStatusBeingEdited(displayedStatus); setEditError(null); }}>Edit</DropdownMenuItem> : null}
                      {canForward ? <DropdownMenuItem onSelect={() => onForward?.(displayedStatus)}>Forward by DM</DropdownMenuItem> : null}
                      {canInteract ? (
                        <>
                          <DropdownMenuItem onSelect={() => void onAction?.(displayedStatus.bookmarked ? "unbookmark" : "bookmark", displayedStatus)}>
                            {displayedStatus.bookmarked ? "Remove bookmark" : "Bookmark"}
                          </DropdownMenuItem>
                          <DropdownMenuItem onSelect={() => void onAction?.(displayedStatus.favourited ? "unfavourite" : "favourite", displayedStatus)}>
                            {displayedStatus.favourited ? "Remove favourite" : "Favourite"}
                          </DropdownMenuItem>
                          {canPin ? (
                            <DropdownMenuItem onSelect={() => void onAction?.(displayedStatus.pinned ? "unpin" : "pin", displayedStatus)}>
                              {displayedStatus.pinned ? "Unpin from profile" : "Pin to profile"}
                            </DropdownMenuItem>
                          ) : null}
                          <DropdownMenuItem onSelect={() => void onAction?.(displayedStatus.reblogged ? "unreblog" : "reblog", displayedStatus)}>
                            {displayedStatus.reblogged ? "Undo boost" : "Boost"}
                          </DropdownMenuItem>
                        </>
                      ) : null}
                      {canDelete ? (
                        <DropdownMenuItem className="text-destructive focus:text-destructive" onSelect={() => setStatusPendingDeletion(displayedStatus)}>
                          Delete
                        </DropdownMenuItem>
                      ) : null}
                    </DropdownMenuContent>
                  </DropdownMenu>
                ) : null}
              </div>
            </article>
          );
        })}
      </div>

      <Dialog open={Boolean(mediaPreview)} onOpenChange={(open) => !open && setMediaPreview(null)}>
        <DialogContent className="max-w-4xl">
          <DialogHeader>
            <DialogTitle>Media</DialogTitle>
            {mediaPreview?.description ? <DialogDescription>{mediaPreview.description}</DialogDescription> : null}
          </DialogHeader>
          {mediaPreview ? (
            <div className="flex justify-center rounded-md bg-background p-2">
              {mediaPreview.type === "image" ? (
                <img className="max-h-[75vh] rounded-md object-contain" src={mediaPreview.url} alt={mediaPreview.description || "Media attachment"} />
              ) : (
                <a className="text-primary hover:underline" href={mediaPreview.url} target="_blank" rel="noreferrer">Open media</a>
              )}
            </div>
          ) : null}
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(statusBeingEdited)} onOpenChange={(open) => !open && setStatusBeingEdited(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Edit post</DialogTitle>
            <DialogDescription>Save changes locally and federate an Update activity.</DialogDescription>
          </DialogHeader>
          {statusBeingEdited ? (
            <ComposeForm
              submitLabel="Save changes"
              submittingLabel="Saving..."
              placeholder="Edit your post"
              isSubmitting={isEditingStatus}
              error={editError}
              initialText={htmlToPlainText(statusBeingEdited.content)}
              initialVisibility={statusBeingEdited.visibility as ComposeValues["visibility"]}
              initialSensitive={statusBeingEdited.sensitive}
              initialSpoilerText={statusBeingEdited.spoiler_text}
              initialMedia={statusBeingEdited.media_attachments?.[0] ?? null}
              initialObjectType={statusObjectType(statusBeingEdited)}
              initialPollOptions={statusBeingEdited.poll?.options.map((option) => option.title)}
              initialPollMultiple={statusBeingEdited.poll?.multiple}
              resetAfterSubmit={false}
              onSubmit={async (values) => {
                setIsEditingStatus(true);
                setEditError(null);
                try {
                  const edited = await onEdit?.(statusBeingEdited, values);
                  if (edited) {
                    setStatusBeingEdited(null);
                  } else {
                    setEditError("Could not edit post.");
                  }
                } catch (caughtError) {
                  setEditError(caughtError instanceof Error ? caughtError.message : "Could not edit post.");
                } finally {
                  setIsEditingStatus(false);
                }
              }}
            />
          ) : null}
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(statusPendingDeletion)} onOpenChange={(open) => !open && setStatusPendingDeletion(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete post?</DialogTitle>
            <DialogDescription>This removes the post from Gargoyle and sends a delete activity to followers.</DialogDescription>
          </DialogHeader>
          {statusPendingDeletion ? (
            <div className="rounded-md border border-border bg-background p-3 text-sm leading-6 text-muted-foreground">
              {htmlToPlainText(statusPendingDeletion.content)}
            </div>
          ) : null}
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline" disabled={isDeleting}>Cancel</Button>
            </DialogClose>
            <Button
              variant="destructive"
              disabled={!statusPendingDeletion || isDeleting}
              onClick={async () => {
                if (!statusPendingDeletion) {
                  return;
                }
                const deleted = await onDelete?.(statusPendingDeletion);
                if (deleted) {
                  setStatusPendingDeletion(null);
                }
              }}
            >
              {isDeleting ? "Deleting..." : "Delete"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

function StatusTypeBadge({ status }: { status: MastodonStatus }) {
  const objectType = statusObjectType(status);
  if (objectType === "Note") {
    return null;
  }
  return <span className="rounded-full bg-primary/10 px-2 py-0.5 text-xs font-medium text-primary">{objectType}</span>;
}

function statusObjectType(status: MastodonStatus): ActivityPubObjectType {
  switch (status.activitypub_type) {
    case "Article":
    case "Page":
    case "Question":
      return status.activitypub_type;
    default:
      return "Note";
  }
}

function StatusMeta({ status }: { status: MastodonStatus }) {
  return (
    <div className="flex items-center gap-1 text-xs text-muted-foreground">
      <span className="rounded-full bg-secondary px-2 py-0.5 capitalize text-secondary-foreground">{status.visibility}</span>
      {status.sensitive ? <span className="rounded-full bg-secondary px-2 py-0.5 text-secondary-foreground">Sensitive</span> : null}
      {status.spoiler_text ? <span className="rounded-full bg-secondary px-2 py-0.5 text-secondary-foreground">CW</span> : null}
    </div>
  );
}

function StatusStats({ status }: { status: MastodonStatus }) {
  const stats = [
    status.replies_count > 0 ? `${status.replies_count} replies` : null,
    status.reblogs_count > 0 ? `${status.reblogs_count} boosts` : null,
    status.favourites_count > 0 ? `${status.favourites_count} favourites` : null,
    status.bookmarked ? "Bookmarked" : null,
    status.favourited ? "Favourited" : null,
    status.reblogged ? "Boosted" : null,
    status.pinned ? "Pinned" : null,
  ].filter(Boolean);

  if (stats.length === 0) {
    return null;
  }

  return <p className="mt-2 text-xs text-muted-foreground">{stats.join(" · ")}</p>;
}

function StatusPoll({ status, onVote }: { status: MastodonStatus; onVote?: (choices: number[]) => Promise<void> | void }) {
  const poll = status.poll;
  const [selected, setSelected] = useState<number[]>(poll?.own_votes ?? []);
  if (!poll) {
    return null;
  }
  const currentPoll = poll;
  const savedVotes = currentPoll.own_votes ?? [];
  const hasSavedVote = savedVotes.length > 0;
  const hasPendingChange = !sameChoices(selected, savedVotes);
  const total = Math.max(currentPoll.votes_count, 1);
  const canVote = Boolean(onVote && !currentPoll.expired);
  function toggle(index: number) {
    if (!canVote) return;
    setSelected((current) => currentPoll.multiple ? (current.includes(index) ? current.filter((item) => item !== index) : [...current, index]) : [index]);
  }
  return (
    <div className="mt-3 space-y-2 rounded-md border border-border bg-background p-3">
      {currentPoll.options.map((option, index) => {
        const percent = Math.round((option.votes_count / total) * 100);
        const isPending = selected.includes(index);
        const isSaved = savedVotes.includes(index);
        const chosen = isPending || isSaved;
        const optionStateClass = isPending
          ? "border-primary/70 bg-primary/10 ring-1 ring-primary/40"
          : isSaved
            ? "border-border bg-secondary/70"
            : "border-border bg-card hover:border-primary/30 hover:bg-secondary/60";
        return (
          <button
            key={`${option.title}-${index}`}
            type="button"
            disabled={!canVote}
            aria-pressed={chosen}
            onClick={() => toggle(index)}
            className={[
              "w-full rounded-md border p-2 text-left transition-colors disabled:cursor-default",
              optionStateClass,
            ].join(" ")}
          >
            <div className="flex items-center justify-between gap-3 text-sm">
              <span className={isPending ? "flex items-center gap-2 font-semibold text-primary" : isSaved ? "flex items-center gap-2 font-medium text-foreground" : "font-medium"}>
                {isPending ? <Check className="size-4 shrink-0" aria-hidden="true" /> : isSaved ? <span className="size-4 shrink-0 rounded-full border border-border bg-background" aria-hidden="true" /> : null}
                {option.title}
              </span>
              <span className="text-xs text-muted-foreground">{percent}%</span>
            </div>
            <div className={chosen ? "mt-2 h-1.5 overflow-hidden rounded-full bg-primary/20" : "mt-2 h-1.5 overflow-hidden rounded-full bg-secondary"}>
              <div className="h-full rounded-full bg-primary/70" style={{ width: `${percent}%` }} />
            </div>
          </button>
        );
      })}
      <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
        <span>{currentPoll.votes_count} votes{currentPoll.expired ? " · closed" : ""}</span>
        {canVote ? (
          <div className="flex items-center gap-2">
            {hasPendingChange ? (
              <Button type="button" size="sm" variant="ghost" onClick={() => setSelected(savedVotes)}>
                {hasSavedVote ? "Cancel change" : "Clear"}
              </Button>
            ) : null}
            <Button type="button" size="sm" variant={hasPendingChange ? "default" : "outline"} disabled={!hasPendingChange || selected.length === 0} onClick={() => onVote?.(selected)}>
              {currentPoll.voted ? "Update vote" : "Vote"}
            </Button>
          </div>
        ) : null}
      </div>
    </div>
  );
}

function sameChoices(a: number[], b: number[]) {
  if (a.length !== b.length) return false;
  const left = [...a].sort((x, y) => x - y);
  const right = [...b].sort((x, y) => x - y);
  return left.every((value, index) => value === right[index]);
}

function StatusMedia({ attachments, onPreview }: { attachments: MastodonMediaAttachment[]; onPreview: (attachment: MastodonMediaAttachment) => void }) {
  if (attachments.length === 0) {
    return null;
  }

  return (
    <div className="mt-3 grid gap-3 sm:grid-cols-2">
      {attachments.map((attachment) => (
        <button
          key={attachment.id}
          type="button"
          className="block overflow-hidden rounded-lg border border-border bg-background text-left"
          onClick={() => onPreview(attachment)}
        >
          {attachment.type === "image" ? (
            <img className="max-h-80 w-full object-cover" src={attachment.preview_url || attachment.url} alt={attachment.description || "Media attachment"} />
          ) : (
            <div className="p-4 text-sm text-muted-foreground">{attachment.type} attachment</div>
          )}
        </button>
      ))}
    </div>
  );
}
