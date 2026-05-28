import { useCallback, useEffect, useMemo, useState } from "react";

import { useAuth } from "@/app/auth-context";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { DirectMessageForm } from "@/features/direct/direct-message-form";
import { EmptyState, FeaturePage, Panel } from "@/features/shared";
import { StatusContent } from "@/features/status/status-content";
import { createMastodonApi } from "@/lib/mastodon-api";
import { accountHref, statusHref } from "@/lib/routes";
import { formatDateTime } from "@/lib/text";
import type { MastodonAccount, MastodonConversation } from "@/types/mastodon";

export function DirectMessagesPage() {
  const { session } = useAuth();
  const [account, setAccount] = useState<MastodonAccount | null>(null);
  const [conversations, setConversations] = useState<MastodonConversation[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [busyConversationId, setBusyConversationId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const api = useMemo(() => (session?.accessToken ? createMastodonApi(session.accessToken) : null), [session?.accessToken]);

  const loadConversations = useCallback(async () => {
    if (!api) return;
    setIsLoading(true);
    setError(null);

    try {
      const [nextAccount, nextConversations] = await Promise.all([api.verifyCredentials(), api.conversations()]);
      setAccount(nextAccount);
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
    <FeaturePage eyebrow="Messages" title="Direct messages" description="Private conversations.">
      <Panel title="New direct message">
        <DirectMessageForm onSent={() => void loadConversations()} />
      </Panel>

      <Panel title="Conversations">
        <div className="mb-5 flex justify-end">
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
                currentAccountId={account?.id}
                isBusy={busyConversationId === conversation.id}
                onMarkRead={markRead}
                onDelete={removeConversation}
              />
            ))}
          </div>
        )}
      </Panel>
    </FeaturePage>
  );
}

type ConversationRowProps = {
  conversation: MastodonConversation;
  currentAccountId?: string;
  isBusy: boolean;
  onMarkRead: (conversation: MastodonConversation) => void;
  onDelete: (conversation: MastodonConversation) => void;
};

function ConversationRow({ conversation, currentAccountId, isBusy, onMarkRead, onDelete }: ConversationRowProps) {
  const participants = conversation.accounts.filter((participant) => participant.id !== currentAccountId);
  const visibleParticipants = participants.length > 0 ? participants : conversation.accounts;
  const participantLabel = visibleParticipants.map((participant) => participant.display_name || participant.username || participant.acct).join(", ");
  const lastStatus = conversation.last_status;

  return (
    <article className="py-4 first:pt-0 last:pb-0">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
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
          <div className="mb-2 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
            <a className="font-medium text-foreground hover:underline" href={accountHref(lastStatus.account.id)}>
              {lastStatus.account.display_name || lastStatus.account.username}
            </a>
            <span>@{lastStatus.account.acct}</span>
            <a className="ml-auto hover:underline" href={statusHref(lastStatus.id)}>
              <time dateTime={lastStatus.created_at}>{formatDateTime(lastStatus.created_at)}</time>
            </a>
          </div>
          <StatusContent html={lastStatus.content} mentions={lastStatus.mentions} />
        </div>
      ) : null}
    </article>
  );
}
