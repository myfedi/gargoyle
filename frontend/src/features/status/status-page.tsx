import { useCallback, useEffect, useMemo, useState } from "react";

import { useAuth } from "@/app/auth-context";
import { Button } from "@/components/ui/button";
import { EmptyState, FeaturePage, Panel } from "@/features/shared";
import { ReplyComposer } from "@/features/status/reply-composer";
import { StatusList } from "@/features/status/status-list";
import { createMastodonApi } from "@/lib/mastodon-api";
import { decodeRouteParam } from "@/lib/routes";
import type { MastodonAccount, MastodonStatus } from "@/types/mastodon";

type StatusPageProps = {
  route: string;
};

export function StatusPage({ route }: StatusPageProps) {
  const { session } = useAuth();
  const statusId = decodeRouteParam(route.split("/")[2]);
  const [status, setStatus] = useState<MastodonStatus | null>(null);
  const [ancestors, setAncestors] = useState<MastodonStatus[]>([]);
  const [descendants, setDescendants] = useState<MastodonStatus[]>([]);
  const [currentAccount, setCurrentAccount] = useState<MastodonAccount | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [deletingStatusId, setDeletingStatusId] = useState<string | null>(null);
  const [replyingTo, setReplyingTo] = useState<MastodonStatus | null>(null);
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
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not load post.");
    } finally {
      setIsLoading(false);
    }
  }, [api, statusId]);

  useEffect(() => {
    void loadStatus();
  }, [loadStatus]);

  async function deleteStatus(statusToDelete: MastodonStatus) {
    if (!api) {
      return false;
    }

    setDeletingStatusId(statusToDelete.id);
    setError(null);

    try {
      await api.deleteStatus(statusToDelete.id);
      window.location.hash = "#/";
      return true;
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not delete post.");
      return false;
    } finally {
      setDeletingStatusId(null);
    }
  }

  async function submitReply(text: string) {
    if (!api || !replyingTo) {
      return;
    }

    setIsReplying(true);
    setReplyError(null);

    try {
      await api.createStatus({ status: text, visibility: "public", in_reply_to_id: replyingTo.id });
      setReplyingTo(null);
      await loadStatus();
    } catch (caughtError) {
      setReplyError(caughtError instanceof Error ? caughtError.message : "Could not post reply.");
    } finally {
      setIsReplying(false);
    }
  }

  const fullThread = [...ancestors, ...(status ? [status] : []), ...descendants];

  return (
    <FeaturePage eyebrow="Post" title="Post" description="Post details and replies.">
      <Panel title="Thread">
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
            currentAccountId={currentAccount?.id}
            emptyTitle="Post not found"
            emptyDescription="No post to show."
            deletingStatusId={deletingStatusId}
            onDelete={deleteStatus}
            onReply={(nextStatus) => {
              setReplyingTo(nextStatus);
              setReplyError(null);
            }}
          />
        ) : (
          <EmptyState title="Post not found" description="No post to show." />
        )}
      </Panel>

      {replyingTo ? (
        <ReplyComposer
          status={replyingTo}
          isSubmitting={isReplying}
          error={replyError}
          onCancel={() => setReplyingTo(null)}
          onSubmit={submitReply}
        />
      ) : null}

      <Button variant="outline" onClick={() => window.history.back()}>Back</Button>
    </FeaturePage>
  );
}
