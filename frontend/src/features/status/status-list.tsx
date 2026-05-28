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
import type { MastodonStatus } from "@/types/mastodon";

type StatusListProps = {
  statuses: MastodonStatus[];
  currentAccountId?: string;
  emptyTitle: string;
  emptyDescription: string;
  deletingStatusId?: string | null;
  onDelete?: (status: MastodonStatus) => Promise<boolean> | boolean;
  onReply?: (status: MastodonStatus) => void;
};

export function StatusList({
  statuses,
  currentAccountId,
  emptyTitle,
  emptyDescription,
  deletingStatusId,
  onDelete,
  onReply,
}: StatusListProps) {
  const [statusPendingDeletion, setStatusPendingDeletion] = useState<MastodonStatus | null>(null);

  if (statuses.length === 0) {
    return <EmptyState title={emptyTitle} description={emptyDescription} />;
  }

  const isDeleting = Boolean(statusPendingDeletion && deletingStatusId === statusPendingDeletion.id);

  return (
    <>
      <div className="divide-y divide-border">
        {statuses.map((status) => {
          const canDelete = Boolean(onDelete && currentAccountId && status.account.id === currentAccountId);
          const canReply = Boolean(onReply);
          return (
            <article key={status.id} className="py-4 first:pt-0 last:pb-0">
              <div className="flex items-start gap-3">
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
                    <a className="text-sm font-semibold hover:underline" href={accountHref(status.account.id)}>
                      {status.account.display_name || status.account.username}
                    </a>
                    <p className="text-xs text-muted-foreground">@{status.account.acct}</p>
                    <a className="ml-auto text-xs text-muted-foreground hover:underline" href={statusHref(status.id)}>
                      <time dateTime={status.created_at}>{formatDateTime(status.created_at)}</time>
                    </a>
                  </div>
                  <div className="mt-2">
                    <StatusContent html={status.content} />
                  </div>
                </div>
                {canDelete || canReply ? (
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button variant="ghost" size="icon" aria-label="Post actions">
                        <MoreHorizontal className="size-4" aria-hidden="true" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      {canReply ? (
                        <DropdownMenuItem onSelect={() => onReply?.(status)}>
                          Reply
                        </DropdownMenuItem>
                      ) : null}
                      {canDelete ? (
                        <DropdownMenuItem
                          className="text-destructive focus:text-destructive"
                          onSelect={() => setStatusPendingDeletion(status)}
                        >
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
