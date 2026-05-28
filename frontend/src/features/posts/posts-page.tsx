import type React from "react";
import { useCallback, useEffect, useMemo, useState } from "react";

import { useAuth } from "@/app/auth-context";
import { Button } from "@/components/ui/button";
import { Tabs } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { EmptyState, FeaturePage, Panel } from "@/features/shared";
import { createMastodonApi } from "@/lib/mastodon-api";
import { formatDateTime, htmlToPlainText } from "@/lib/text";
import type { MastodonStatus } from "@/types/mastodon";

const maxPostLength = 500;
const timelineTabs = [
  { value: "home", label: "Home" },
  { value: "local", label: "Local" },
  { value: "global", label: "Global" },
] as const;

type TimelineTab = (typeof timelineTabs)[number]["value"];

export function PostsPage() {
  const { session } = useAuth();
  const [activeTimeline, setActiveTimeline] = useState<TimelineTab>("home");
  const [statuses, setStatuses] = useState<MastodonStatus[]>([]);
  const [statusText, setStatusText] = useState("");
  const [isLoading, setIsLoading] = useState(true);
  const [isPosting, setIsPosting] = useState(false);
  const [timelineError, setTimelineError] = useState<string | null>(null);
  const [publishError, setPublishError] = useState<string | null>(null);

  const api = useMemo(() => (session?.accessToken ? createMastodonApi(session.accessToken) : null), [session?.accessToken]);
  const remaining = maxPostLength - statusText.length;

  const loadTimeline = useCallback(
    async (timeline: TimelineTab, options: { silent?: boolean } = {}) => {
      if (!api) {
        return;
      }

      if (!options.silent) {
        setIsLoading(true);
      }
      setTimelineError(null);

      try {
        const nextStatuses = timeline === "home" ? await api.homeTimeline() : await api.publicTimeline();
        setStatuses(nextStatuses);
      } catch (caughtError) {
        setTimelineError(caughtError instanceof Error ? caughtError.message : "Could not load timeline.");
      } finally {
        setIsLoading(false);
      }
    },
    [api],
  );

  useEffect(() => {
    void loadTimeline(activeTimeline);
  }, [activeTimeline, loadTimeline]);

  async function submitPost(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!api || !statusText.trim() || remaining < 0) {
      return;
    }

    setIsPosting(true);
    setPublishError(null);

    try {
      const createdStatus = await api.createStatus({ status: statusText.trim(), visibility: "public" });
      if (activeTimeline === "home") {
        setStatuses((current) => [createdStatus, ...current]);
      }
      setStatusText("");
    } catch (caughtError) {
      setPublishError(caughtError instanceof Error ? caughtError.message : "Could not publish post.");
    } finally {
      setIsPosting(false);
    }
  }

  return (
    <FeaturePage eyebrow="Timeline" title="Timeline" description="Write posts and read recent activity.">
      <Panel title="New post">
        <form className="space-y-4" onSubmit={(event) => void submitPost(event)}>
          <Textarea
            value={statusText}
            onChange={(event) => setStatusText(event.target.value)}
            placeholder="What would you like to share?"
            aria-label="Post content"
            rows={6}
          />
          {publishError ? (
            <p className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive" role="alert">
              {publishError}
            </p>
          ) : null}
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

      <Panel title="Posts">
        <div className="mb-5 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <Tabs value={activeTimeline} onValueChange={setActiveTimeline} items={[...timelineTabs]} />
          <Button variant="outline" size="sm" onClick={() => void loadTimeline(activeTimeline)} disabled={isLoading}>
            {isLoading ? "Refreshing..." : "Refresh"}
          </Button>
        </div>

        {timelineError ? (
          <p className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive" role="alert">
            {timelineError}
          </p>
        ) : isLoading ? (
          <div className="space-y-3" aria-label="Loading posts">
            {[0, 1, 2].map((item) => (
              <div key={item} className="h-24 animate-pulse rounded-md bg-secondary" />
            ))}
          </div>
        ) : statuses.length === 0 ? (
          <EmptyState title="No posts" description="Nothing to show here yet." />
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
