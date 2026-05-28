import type React from "react";
import { useEffect, useMemo, useState } from "react";

import { useAuth } from "@/app/auth-context";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { EmptyState, FeaturePage, Panel } from "@/features/shared";
import { createMastodonApi } from "@/lib/mastodon-api";
import { formatDateTime, htmlToPlainText } from "@/lib/text";
import type { MastodonStatus } from "@/types/mastodon";

const maxPostLength = 500;

export function PostsPage() {
  const { session, signOut } = useAuth();
  const [statuses, setStatuses] = useState<MastodonStatus[]>([]);
  const [statusText, setStatusText] = useState("");
  const [isLoading, setIsLoading] = useState(true);
  const [isPosting, setIsPosting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const api = useMemo(() => (session?.accessToken ? createMastodonApi(session.accessToken) : null), [session?.accessToken]);
  const remaining = maxPostLength - statusText.length;

  useEffect(() => {
    if (!api) {
      return;
    }

    let cancelled = false;
    setIsLoading(true);
    setError(null);

    api
      .homeTimeline()
      .then((timeline) => {
        if (!cancelled) {
          setStatuses(timeline);
        }
      })
      .catch((caughtError: unknown) => {
        if (!cancelled) {
          setError(caughtError instanceof Error ? caughtError.message : "Could not load posts.");
        }
      })
      .finally(() => {
        if (!cancelled) {
          setIsLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [api]);

  async function submitPost(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!api || !statusText.trim() || remaining < 0) {
      return;
    }

    setIsPosting(true);
    setError(null);

    try {
      const createdStatus = await api.createStatus({ status: statusText.trim(), visibility: "public" });
      setStatuses((current) => [createdStatus, ...current]);
      setStatusText("");
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not publish post.");
    } finally {
      setIsPosting(false);
    }
  }

  return (
    <FeaturePage
      eyebrow="Publishing"
      title="Posts"
      description="Write local notes through Gargoyle's Mastodon-compatible API. Remote content is rendered as plain text until a sanitizer is in place."
      status="ready"
    >
      <div className="grid gap-6 xl:grid-cols-[minmax(0,42rem)_1fr]">
        <Panel title="New post" description="Public posting is wired to POST /api/v1/statuses.">
          <form className="space-y-4" onSubmit={(event) => void submitPost(event)}>
            <Textarea
              value={statusText}
              onChange={(event) => setStatusText(event.target.value)}
              placeholder="What would you like to share?"
              aria-label="Post content"
              rows={6}
            />
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <p className={remaining < 0 ? "text-sm text-destructive" : "text-sm text-muted-foreground"}>
                {remaining} characters remaining
              </p>
              <Button type="submit" disabled={isPosting || !statusText.trim() || remaining < 0}>
                {isPosting ? "Publishing..." : "Publish"}
              </Button>
            </div>
          </form>
        </Panel>

        <Panel title="Session" description="If token validation fails, sign out and authorize again.">
          <Button variant="outline" onClick={signOut}>Sign out</Button>
        </Panel>
      </div>

      <Panel title="Home timeline" description="Loaded from GET /api/v1/timelines/home.">
        {error ? (
          <p className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive" role="alert">
            {error}
          </p>
        ) : isLoading ? (
          <div className="space-y-3" aria-label="Loading posts">
            {[0, 1, 2].map((item) => (
              <div key={item} className="h-24 animate-pulse rounded-md bg-secondary" />
            ))}
          </div>
        ) : statuses.length === 0 ? (
          <EmptyState title="No posts yet" description="Publish the first local note or wait for home timeline data to arrive." />
        ) : (
          <div className="divide-y divide-border">
            {statuses.map((status) => (
              <article key={status.id} className="py-4 first:pt-0 last:pb-0">
                <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
                  <h3 className="text-sm font-semibold">{status.account.display_name || status.account.username}</h3>
                  <p className="text-xs text-muted-foreground">@{status.account.acct}</p>
                  <time className="ml-auto text-xs text-muted-foreground" dateTime={status.created_at}>
                    {formatDateTime(status.created_at)}
                  </time>
                </div>
                <p className="mt-2 whitespace-pre-wrap text-sm leading-6">{htmlToPlainText(status.content)}</p>
              </article>
            ))}
          </div>
        )}
      </Panel>
    </FeaturePage>
  );
}
