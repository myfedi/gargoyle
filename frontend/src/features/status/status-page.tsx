import { useCallback, useEffect, useMemo, useState } from "react";

import { useAuth } from "@/app/auth-context";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { DirectMessageForm } from "@/features/direct/direct-message-form";
import { EmptyState, Panel } from "@/features/shared";
import type { ComposeValues } from "@/features/status/compose-form";
import { ReplyComposer } from "@/features/status/reply-composer";
import { optimisticStatusAction, replaceOneStatus, runStatusAction } from "@/features/status/status-actions";
import { StatusList, type StatusAction } from "@/features/status/status-list";
import { createMastodonApi } from "@/lib/mastodon-api";
import { decodeRouteParam } from "@/lib/routes";
import type { MastodonAccount, MastodonStatus } from "@/types/mastodon";

type StatusPageProps = {
  route: string;
};

function uniqueStatuses(statuses: MastodonStatus[]) {
  const seen = new Set<string>();
  return statuses.filter((status) => {
    if (seen.has(status.id)) {
      return false;
    }
    seen.add(status.id);
    return true;
  });
}

export function StatusPage({ route }: StatusPageProps) {
  const { session } = useAuth();
  const statusId = decodeRouteParam(route.split("/")[2]);
  const [status, setStatus] = useState<MastodonStatus | null>(null);
  const [ancestors, setAncestors] = useState<MastodonStatus[]>([]);
  const [descendants, setDescendants] = useState<MastodonStatus[]>([]);
  const [contextWarnings, setContextWarnings] = useState<string[]>([]);
  const [currentAccount, setCurrentAccount] = useState<MastodonAccount | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [deletingStatusId, setDeletingStatusId] = useState<string | null>(null);
  const [actingStatusId, setActingStatusId] = useState<string | null>(null);
  const [replyingTo, setReplyingTo] = useState<MastodonStatus | null>(null);
  const [forwardingStatus, setForwardingStatus] = useState<MastodonStatus | null>(null);
  const [isReplying, setIsReplying] = useState(false);
  const [replyError, setReplyError] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const api = useMemo(() => (session?.accessToken ? createMastodonApi(session.accessToken) : null), [session?.accessToken]);

  const loadStatus = useCallback(async () => {
    if (!api || !statusId) {
      return;
    }

    setIsLoading(true);
    setError(null);

    try {
      const [nextCurrentAccount, nextStatus, context] = await Promise.all([
        api.verifyCredentials(),
        api.status(statusId),
        api.statusContext(statusId),
      ]);
      setCurrentAccount(nextCurrentAccount);
      setStatus(nextStatus);
      setAncestors(context.ancestors);
      setDescendants(context.descendants);
      setContextWarnings(context.warnings ?? []);
    } catch (caughtError) {
      setContextWarnings([]);
      setError(caughtError instanceof Error ? caughtError.message : "Could not load post.");
    } finally {
      setIsLoading(false);
    }
  }, [api, statusId]);

  useEffect(() => {
    setStatus(null);
    setAncestors([]);
    setDescendants([]);
    setContextWarnings([]);
    setReplyingTo(null);
    setReplyError(null);
    setForwardingStatus(null);
    setActingStatusId(null);
    setDeletingStatusId(null);
  }, [statusId]);

  useEffect(() => {
    void loadStatus();
  }, [loadStatus]);

  async function runAction(action: StatusAction, statusToUpdate: MastodonStatus) {
    if (!api) {
      return;
    }

    setActingStatusId(statusToUpdate.id);
    setError(null);

    const optimisticStatus = optimisticStatusAction(statusToUpdate, action);
    setStatus((current) => current ? replaceOneStatus(current, optimisticStatus) : current);
    setAncestors((current) => current.map((item) => replaceOneStatus(item, optimisticStatus)));
    setDescendants((current) => current.map((item) => replaceOneStatus(item, optimisticStatus)));

    try {
      const nextStatus = await runStatusAction(api, action, statusToUpdate);
      setStatus((current) => current ? replaceOneStatus(current, nextStatus) : current);
      setAncestors((current) => current.map((item) => replaceOneStatus(item, nextStatus)));
      setDescendants((current) => current.map((item) => replaceOneStatus(item, nextStatus)));
    } catch (caughtError) {
      setStatus((current) => current ? replaceOneStatus(current, statusToUpdate) : current);
      setAncestors((current) => current.map((item) => replaceOneStatus(item, statusToUpdate)));
      setDescendants((current) => current.map((item) => replaceOneStatus(item, statusToUpdate)));
      setError(caughtError instanceof Error ? caughtError.message : "Could not update post.");
    } finally {
      setActingStatusId(null);
    }
  }

  async function deleteStatus(statusToDelete: MastodonStatus) {
    if (!api) {
      return false;
    }

    setDeletingStatusId(statusToDelete.id);
    setError(null);

    try {
      await api.deleteStatus(statusToDelete.id);
      globalThis.location.hash = "#/";
      return true;
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not delete post.");
      return false;
    } finally {
      setDeletingStatusId(null);
    }
  }

  async function votePoll(statusToVote: MastodonStatus, choices: number[]) {
    if (!api) return;
    setError(null);
    try {
      const poll = await api.votePoll(statusToVote.poll?.id ?? statusToVote.id, choices);
      const applyPoll = (item: MastodonStatus) => item.id === statusToVote.id ? { ...item, poll } : item;
      setStatus((current) => current ? applyPoll(current) : current);
      setAncestors((current) => current.map(applyPoll));
      setDescendants((current) => current.map(applyPoll));
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not vote in poll.");
    }
  }

  async function editStatus(statusToEdit: MastodonStatus, values: ComposeValues) {
    if (!api) {
      return false;
    }

    setError(null);

    try {
      const updated = await api.updateStatus(statusToEdit.id, {
        status: values.status,
        visibility: values.visibility,
        sensitive: values.sensitive,
        spoiler_text: values.spoilerText,
        media_ids: values.mediaIds,
        activitypub_type: values.objectType,
        poll: values.objectType === "Question" ? { options: values.pollOptions, expires_in: values.pollExpiresIn, multiple: values.pollMultiple } : undefined,
      });
      setStatus((current) => current?.id === updated.id ? updated : current);
      setAncestors((current) => current.map((item) => item.id === updated.id ? updated : item));
      setDescendants((current) => current.map((item) => item.id === updated.id ? updated : item));
      return true;
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not edit post.");
      return false;
    }
  }

  async function submitReply(values: ComposeValues) {
    if (!api || !replyingTo) {
      return;
    }

    setIsReplying(true);
    setReplyError(null);

    try {
      await api.createStatus({
        status: values.status,
        visibility: values.visibility,
        sensitive: values.sensitive,
        spoiler_text: values.spoilerText,
        media_ids: values.mediaIds,
        activitypub_type: values.objectType,
        poll: values.objectType === "Question" ? { options: values.pollOptions, expires_in: values.pollExpiresIn, multiple: values.pollMultiple } : undefined,
        in_reply_to_id: replyingTo.id,
      });
      setReplyingTo(null);
      await loadStatus();
    } catch (caughtError) {
      setReplyError(caughtError instanceof Error ? caughtError.message : "Could not post reply.");
    } finally {
      setIsReplying(false);
    }
  }

  const fullThread = useMemo(
    () => uniqueStatuses([...ancestors, ...(status ? [status] : []), ...descendants]),
    [ancestors, descendants, status],
  );

  const replyTargets = useMemo(() => {
    const statusesById = new Map(fullThread.map((threadStatus) => [threadStatus.id, threadStatus]));
    const targets = new Map<string, MastodonStatus>();
    for (const threadStatus of fullThread) {
      if (!threadStatus.in_reply_to_id) {
        continue;
      }
      const target = statusesById.get(threadStatus.in_reply_to_id);
      if (target) {
        targets.set(threadStatus.id, target);
      }
    }
    return targets;
  }, [fullThread]);

  return (
    <section className="mx-auto max-w-2xl space-y-5">
      <Panel title="Thread">
        {contextWarnings.length > 0 ? (
          <div className="mb-4 space-y-2" role="status" aria-live="polite">
            {contextWarnings.map((warning) => (
              <p key={warning} className="rounded-md border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-sm text-amber-700 dark:text-amber-300">
                {warning}
              </p>
            ))}
          </div>
        ) : null}
        {error ? (
          <p className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive" role="alert">
            {error}
          </p>
        ) : isLoading ? (
          <div className="space-y-3">
            {[0, 1, 2].map((item) => <div key={item} className="h-24 animate-pulse rounded-md bg-secondary" />)}
          </div>
        ) : fullThread.length > 0 ? (
          <StatusList
            statuses={fullThread}
            replyTargets={replyTargets}
            currentAccountId={currentAccount?.id}
            emptyTitle="Post not found"
            emptyDescription="No post to show."
            deletingStatusId={deletingStatusId}
            actingStatusId={actingStatusId}
            onDelete={deleteStatus}
            onEdit={editStatus}
            onAction={runAction}
            onVotePoll={votePoll}
            onForward={setForwardingStatus}
            onReply={(nextStatus) => {
              setReplyingTo(nextStatus);
              setReplyError(null);
            }}
            renderAfterStatus={(threadStatus) => replyingTo?.id === threadStatus.id ? (
              <ReplyComposer
                status={replyingTo}
                isSubmitting={isReplying}
                error={replyError}
                onCancel={() => setReplyingTo(null)}
                onSubmit={submitReply}
              />
            ) : null}
          />
        ) : (
          <EmptyState title="Post not found" description="No post to show." />
        )}
      </Panel>

      <Button variant="outline" onClick={() => globalThis.history.back()}>Back</Button>
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
