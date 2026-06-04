import { useCallback, useEffect, useMemo, useState } from "react";

import { useAuth } from "@/app/auth-context";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { DirectMessageForm } from "@/features/direct/direct-message-form";
import { EmptyState, Panel } from "@/features/shared";
import { StatusContent } from "@/features/status/status-content";
import { createMastodonApi } from "@/lib/mastodon-api";
import { accountHref, statusHref } from "@/lib/routes";
import { formatDateTime } from "@/lib/text";
import type { MastodonConversation } from "@/types/mastodon";

export function DirectMessagesPage() {
  const { session } = useAuth();
  const [conversations, setConversations] = useState<MastodonConversation[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [busyConversationId, setBusyConversationId] = useState<string | null>(null);
  const [isComposerOpen, setIsComposerOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const api = useMemo(() => (session?.accessToken ? createMastodonApi(session.accessToken) : null), [session?.accessToken]);

  const loadConversations = useCallback(async () => {
    if (!api) return;
    setIsLoading(true);
    setError(null);

    try {
      const nextConversations = await api.conversations();
      setConversations(nextConversations);
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not load direct messages.");
    } finally {
      setIsLoading(false);
    }
  }, [api]);

  useEffect(() => {
    void loadConversations();
  }, [loadConversations]);

  async function markRead(conversation: MastodonConversation) {
    if (!api) return;
    setBusyConversationId(conversation.id);
    setError(null);

    try {
      const nextConversation = await api.markConversationRead(conversation.id);
      setConversations((current) => current.map((item) => item.id === conversation.id ? nextConversation : item));
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not mark conversation read.");
    } finally {
      setBusyConversationId(null);
    }
  }

  async function removeConversation(conversation: MastodonConversation) {
    if (!api) return;
    setBusyConversationId(conversation.id);
    setError(null);

    try {
      await api.deleteConversation(conversation.id);
      setConversations((current) => current.filter((item) => item.id !== conversation.id));
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not delete conversation.");
    } finally {
      setBusyConversationId(null);
    }
  }

  return (
    <section className="space-y-6">
      <Panel title="Conversations">
        <div className="mb-5 flex flex-wrap justify-end gap-2">
          <Button size="sm" onClick={() => setIsComposerOpen(true)}>New direct message</Button>
          <Button variant="outline" size="sm" onClick={() => void loadConversations()} disabled={isLoading}>Refresh</Button>
        </div>

        {error ? <p className="mb-4 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive" role="alert">{error}</p> : null}

        {isLoading ? (
          <div className="space-y-3">{[0, 1, 2].map((item) => <div key={item} className="h-24 animate-pulse rounded-md bg-secondary" />)}</div>
        ) : conversations.length === 0 ? (
          <EmptyState title="No conversations" description="Direct conversations will appear here." />
        ) : (
          <div className="divide-y divide-border">
            {conversations.map((conversation) => (
              <ConversationRow
                key={conversation.id}
                conversation={conversation}
                isBusy={busyConversationId === conversation.id}
                onMarkRead={markRead}
                onDelete={removeConversation}
              />
            ))}
          </div>
        )}
      </Panel>

      <Dialog open={isComposerOpen} onOpenChange={setIsComposerOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>New direct message</DialogTitle>
          </DialogHeader>
          <DirectMessageForm
            onCancel={() => setIsComposerOpen(false)}
            onSent={() => {
              setIsComposerOpen(false);
              void loadConversations();
            }}
          />
        </DialogContent>
      </Dialog>
    </section>
  );
}

type ConversationRowProps = {
  conversation: MastodonConversation;
  isBusy: boolean;
  onMarkRead: (conversation: MastodonConversation) => void;
  onDelete: (conversation: MastodonConversation) => void;
};

function ConversationRow({ conversation, isBusy, onMarkRead, onDelete }: ConversationRowProps) {
  const visibleParticipants = conversation.accounts;
  const participantLabel = visibleParticipants.map((participant) => participant.display_name || participant.username || participant.acct).join(", ");
  const lastStatus = conversation.last_status;

  return (
    <article className="py-4 first:pt-0 last:pb-0">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="flex min-w-0 gap-3">
          <ParticipantAvatarStack participants={visibleParticipants} />
          <div className="min-w-0 space-y-2">
            <div className="flex flex-wrap items-center gap-2">
              <h2 className="text-sm font-semibold">{participantLabel || "Conversation"}</h2>
              {conversation.unread ? <Badge variant="secondary">Unread</Badge> : null}
            </div>
            {visibleParticipants.length > 0 ? (
              <div className="flex flex-wrap gap-2 text-sm text-muted-foreground">
                {visibleParticipants.map((participant) => (
                  <a key={participant.id} className="hover:text-foreground hover:underline" href={accountHref(participant.id)}>
                    @{participant.acct}
                  </a>
                ))}
              </div>
            ) : null}
          </div>
        </div>
        <div className="flex gap-2">
          {conversation.unread ? (
            <Button variant="outline" size="sm" disabled={isBusy} onClick={() => onMarkRead(conversation)}>
              {isBusy ? "Updating..." : "Mark read"}
            </Button>
          ) : null}
          <Button variant="ghost" size="sm" disabled={isBusy} onClick={() => onDelete(conversation)}>
            {isBusy ? "Updating..." : "Delete"}
          </Button>
        </div>
      </div>

      {lastStatus ? (
        <div className="mt-3 rounded-md border border-border bg-background p-3">
          <div className="mb-2 flex items-center gap-2 text-xs text-muted-foreground">
            <AccountAvatar account={lastStatus.account} className="size-7 text-xs" />
            <div className="min-w-0 flex-1">
              <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                <a className="font-medium text-foreground hover:underline" href={accountHref(lastStatus.account.id)}>
                  {lastStatus.account.display_name || lastStatus.account.username}
                </a>
                <span>@{lastStatus.account.acct}</span>
                <a className="ml-auto hover:underline" href={statusHref(lastStatus.id)}>
                  <time dateTime={lastStatus.created_at}>{formatDateTime(lastStatus.created_at)}</time>
                </a>
              </div>
            </div>
          </div>
          <StatusContent html={lastStatus.content} mentions={lastStatus.mentions} />
        </div>
      ) : null}
    </article>
  );
}

function ParticipantAvatarStack({ participants }: { participants: MastodonConversation["accounts"] }) {
  if (participants.length === 0) {
    return <div className="size-10 shrink-0 rounded-full border border-border bg-secondary" aria-hidden="true" />;
  }

  return (
    <div className="flex shrink-0 -space-x-2" aria-hidden="true">
      {participants.slice(0, 3).map((participant) => (
        <AccountAvatar key={participant.id} account={participant} className="size-10 ring-2 ring-card" />
      ))}
    </div>
  );
}

function AccountAvatar({ account, className }: { account: MastodonConversation["accounts"][number]; className: string }) {
  const avatar = account.avatar_static || account.avatar;

  if (avatar) {
    return <img className={`${className} rounded-full border border-border object-cover`} src={avatar} alt="" aria-hidden="true" />;
  }

  return (
    <div className={`${className} flex items-center justify-center rounded-full border border-border bg-secondary font-semibold uppercase text-secondary-foreground`} aria-hidden="true">
      {accountInitials(account)}
    </div>
  );
}

function accountInitials(account: MastodonConversation["accounts"][number]) {
  const label = account.display_name || account.username || account.acct || "?";
  return label.trim().slice(0, 1) || "?";
}
