import { MoreHorizontal } from "lucide-react";
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
import { StatusContent } from "@/features/status/status-content";
import { accountHref, statusHref } from "@/lib/routes";
import { formatDateTime, htmlToPlainText } from "@/lib/text";
import type { MastodonMediaAttachment, MastodonStatus } from "@/types/mastodon";

export type StatusAction = "bookmark" | "unbookmark" | "favourite" | "unfavourite" | "reblog" | "unreblog";

type StatusListProps = {
  statuses: MastodonStatus[];
  currentAccountId?: string;
  emptyTitle: string;
  emptyDescription: string;
  deletingStatusId?: string | null;
  actingStatusId?: string | null;
  onDelete?: (status: MastodonStatus) => Promise<boolean> | boolean;
  onReply?: (status: MastodonStatus) => void;
  onForward?: (status: MastodonStatus) => void;
  onAction?: (action: StatusAction, status: MastodonStatus) => Promise<void> | void;
};

export function StatusList({
  statuses,
  currentAccountId,
  emptyTitle,
  emptyDescription,
  deletingStatusId,
  actingStatusId,
  onDelete,
  onReply,
  onForward,
  onAction,
}: StatusListProps) {
  const [statusPendingDeletion, setStatusPendingDeletion] = useState<MastodonStatus | null>(null);
  const [mediaPreview, setMediaPreview] = useState<MastodonMediaAttachment | null>(null);

  if (statuses.length === 0) {
    return <EmptyState title={emptyTitle} description={emptyDescription} />;
  }

  const isDeleting = Boolean(statusPendingDeletion && deletingStatusId === statusPendingDeletion.id);

  return (
    <>
      <div className="divide-y divide-border">
        {statuses.map((status) => {
          const displayedStatus = status.reblog ?? status;
          const canDelete = Boolean(onDelete && currentAccountId && displayedStatus.account.id === currentAccountId);
          const canReply = Boolean(onReply);
          const canForward = Boolean(onForward);
          const canInteract = Boolean(onAction);
          const isActing = actingStatusId === displayedStatus.id;
          return (
            <article key={status.id} className="py-4 first:pt-0 last:pb-0">
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
                    <StatusMeta status={displayedStatus} />
                    <a className="ml-auto text-xs text-muted-foreground hover:underline" href={statusHref(displayedStatus.id)}>
                      <time dateTime={displayedStatus.created_at}>{formatDateTime(displayedStatus.created_at)}</time>
                    </a>
                  </div>
                  {displayedStatus.spoiler_text ? <p className="mt-2 text-sm font-medium">{displayedStatus.spoiler_text}</p> : null}
                  <div className="mt-2">
                    <StatusContent html={displayedStatus.content} mentions={displayedStatus.mentions} />
                  </div>
                  <StatusStats status={displayedStatus} />
                  <StatusMedia attachments={displayedStatus.media_attachments ?? []} onPreview={setMediaPreview} />
                </div>
                {canDelete || canReply || canForward || canInteract ? (
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button variant="ghost" size="icon" aria-label="Post actions" disabled={isActing}>
                        <MoreHorizontal className="size-4" aria-hidden="true" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      {canReply ? <DropdownMenuItem onSelect={() => onReply?.(displayedStatus)}>Reply</DropdownMenuItem> : null}
                      {canForward ? <DropdownMenuItem onSelect={() => onForward?.(displayedStatus)}>Forward by DM</DropdownMenuItem> : null}
                      {canInteract ? (
                        <>
                          <DropdownMenuItem onSelect={() => void onAction?.(displayedStatus.bookmarked ? "unbookmark" : "bookmark", displayedStatus)}>
                            {displayedStatus.bookmarked ? "Remove bookmark" : "Bookmark"}
                          </DropdownMenuItem>
                          <DropdownMenuItem onSelect={() => void onAction?.(displayedStatus.favourited ? "unfavourite" : "favourite", displayedStatus)}>
                            {displayedStatus.favourited ? "Remove favourite" : "Favourite"}
                          </DropdownMenuItem>
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
  ].filter(Boolean);

  if (stats.length === 0) {
    return null;
  }

  return <p className="mt-2 text-xs text-muted-foreground">{stats.join(" · ")}</p>;
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
