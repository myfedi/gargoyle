import { useCallback, useEffect, useMemo, useRef, useState, type MouseEvent } from "react";

import { useAuth } from "@/app/auth-context";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Tabs } from "@/components/ui/tabs";
import { DirectMessageForm } from "@/features/direct/direct-message-form";
import { EmptyState } from "@/features/shared";
import { ComposeForm, type ComposeValues } from "@/features/status/compose-form";
import { ReplyComposer } from "@/features/status/reply-composer";
import { replaceStatus, runStatusAction } from "@/features/status/status-actions";
import { StatusList, type StatusAction } from "@/features/status/status-list";
import { createMastodonApi } from "@/lib/mastodon-api";
import type { MastodonAccount, MastodonStatus } from "@/types/mastodon";

const timelineTabs = [
  { value: "home", label: "Home" },
  { value: "local", label: "Local" },
  { value: "global", label: "Global" },
] as const;

const timelineLimit = 20;

type TimelineTab = (typeof timelineTabs)[number]["value"];

type TimelineCacheEntry = {
  timeline: TimelineTab;
  statuses: MastodonStatus[];
  currentAccount: MastodonAccount | null;
  hasMore: boolean;
  scrollY: number;
};

type PostsPageProps = {
  route?: string;
};

const timelineCacheStorageKey = "gargoyle.timelineCache.v2";
const timelineCache: Partial<Record<TimelineTab, TimelineCacheEntry>> = readTimelineCache();

function readTimelineCache(): Partial<Record<TimelineTab, TimelineCacheEntry>> {
  if (typeof window === "undefined") {
    return {};
  }
  try {
    const raw = window.sessionStorage.getItem(timelineCacheStorageKey);
    return raw ? JSON.parse(raw) : {};
  } catch {
    return {};
  }
}

function writeTimelineCache() {
  try {
    window.sessionStorage.setItem(timelineCacheStorageKey, JSON.stringify(timelineCache));
  } catch {
    // Ignore storage quota/private-mode failures. The in-memory cache still works.
  }
}

function validTimelineCache(timeline: TimelineTab) {
  const cached = timelineCache[timeline];
  return cached?.timeline === timeline ? cached : undefined;
}

export function PostsPage({ route = "/" }: PostsPageProps) {
  const { session } = useAuth();
  const timelineFromRoute = routeToTimeline(route);
  const initialCache = validTimelineCache(timelineFromRoute);
  const [activeTimeline, setActiveTimeline] = useState<TimelineTab>(timelineFromRoute);
  const [statuses, setStatuses] = useState<MastodonStatus[]>(initialCache?.statuses ?? []);
  const [currentAccount, setCurrentAccount] = useState<MastodonAccount | null>(initialCache?.currentAccount ?? null);
  const [hasMore, setHasMore] = useState(initialCache?.hasMore ?? true);
  const [isLoading, setIsLoading] = useState(true);
  const [isPosting, setIsPosting] = useState(false);
  const [isLoadingMore, setIsLoadingMore] = useState(false);
  const [actingStatusId, setActingStatusId] = useState<string | null>(null);
  const [timelineError, setTimelineError] = useState<string | null>(null);
  const [publishError, setPublishError] = useState<string | null>(null);
  const [replyingTo, setReplyingTo] = useState<MastodonStatus | null>(null);
  const [forwardingStatus, setForwardingStatus] = useState<MastodonStatus | null>(null);
  const [isReplying, setIsReplying] = useState(false);
  const [replyError, setReplyError] = useState<string | null>(null);

  const restoredTimelineRef = useRef<TimelineTab | null>(null);
  const pendingScrollYRef = useRef<number | null>(initialCache?.scrollY ?? null);
  const api = useMemo(() => (session?.accessToken ? createMastodonApi(session.accessToken) : null), [session?.accessToken]);

  const searchKnownAccounts = useCallback(async (query: string) => {
    if (!api) return [];
    return api.searchKnownAccounts(query);
  }, [api]);

  const saveCurrentTimeline = useCallback((scrollY = window.scrollY) => {
    timelineCache[activeTimeline] = { timeline: activeTimeline, statuses, currentAccount, hasMore, scrollY };
    writeTimelineCache();
  }, [activeTimeline, currentAccount, hasMore, statuses]);

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
        const [nextCurrentAccount, nextStatuses] = await Promise.all([
          api.verifyCredentials(),
          loadTimelinePage(timeline),
        ]);
        setCurrentAccount(nextCurrentAccount);
        setStatuses(nextStatuses);
        setHasMore(nextStatuses.length >= timelineLimit);
      } catch (caughtError) {
        setTimelineError(caughtError instanceof Error ? caughtError.message : "Could not load timeline.");
      } finally {
        setIsLoading(false);
      }
    },
    [api],
  );

  useEffect(() => {
    if (activeTimeline !== timelineFromRoute) {
      setActiveTimeline(timelineFromRoute);
    }
  }, [activeTimeline, timelineFromRoute]);

  useEffect(() => {
    return () => saveCurrentTimeline();
  }, [saveCurrentTimeline]);

  useEffect(() => {
    const save = () => saveCurrentTimeline();
    window.addEventListener("pagehide", save);
    return () => window.removeEventListener("pagehide", save);
  }, [saveCurrentTimeline]);

  useEffect(() => {
    const cached = validTimelineCache(activeTimeline);
    if (cached?.statuses.length) {
      setStatuses(cached.statuses);
      setCurrentAccount(cached.currentAccount);
      setHasMore(cached.hasMore);
      setIsLoading(false);
      setTimelineError(null);
      if (restoredTimelineRef.current !== activeTimeline) {
        restoredTimelineRef.current = activeTimeline;
        pendingScrollYRef.current = cached.scrollY;
      }
      return;
    }

    void loadTimeline(activeTimeline);
  }, [activeTimeline, loadTimeline]);

  useEffect(() => {
    if (isLoading || statuses.length === 0 || pendingScrollYRef.current === null) {
      return;
    }

    const top = pendingScrollYRef.current;
    pendingScrollYRef.current = null;

    requestAnimationFrame(() => {
      requestAnimationFrame(() => window.scrollTo({ top, behavior: "auto" }));
    });
    const timeouts = [50, 150, 350, 750].map((delay) => window.setTimeout(() => window.scrollTo({ top, behavior: "auto" }), delay));
    return () => timeouts.forEach((timeout) => window.clearTimeout(timeout));
  }, [isLoading, statuses.length]);

  async function loadMore() {
    if (!api || statuses.length === 0) {
      return;
    }

    setIsLoadingMore(true);
    setTimelineError(null);

    try {
      const nextStatuses = await loadTimelinePage(activeTimeline, statuses.at(-1)?.id);
      setHasMore(nextStatuses.length >= timelineLimit);
      if (nextStatuses.length === 0) {
        setTimelineError("No more posts to load.");
        return;
      }
      setStatuses((current) => {
        const seen = new Set(current.map((status) => status.id));
        const merged = [...current, ...nextStatuses.filter((status) => !seen.has(status.id))];
        timelineCache[activeTimeline] = { timeline: activeTimeline, statuses: merged, currentAccount, hasMore: nextStatuses.length >= timelineLimit, scrollY: window.scrollY };
        writeTimelineCache();
        return merged;
      });
    } catch (caughtError) {
      setTimelineError(caughtError instanceof Error ? caughtError.message : "Could not load more posts.");
    } finally {
      setIsLoadingMore(false);
    }
  }

  function navigateTimeline(timeline: TimelineTab) {
    saveCurrentTimeline();
    setActiveTimeline(timeline);
    window.location.hash = timeline;
  }

  function loadTimelinePage(timeline: TimelineTab, maxId?: string) {
    if (!api) {
      return Promise.resolve([]);
    }

    const options = { limit: timelineLimit, maxId };
    if (timeline === "home") {
      return api.homeTimeline(options);
    }
    if (timeline === "local") {
      return api.publicTimeline({ ...options, local: true });
    }
    return api.publicTimeline(options);
  }

  async function runAction(action: StatusAction, status: MastodonStatus) {
    if (!api) {
      return;
    }

    setActingStatusId(status.id);
    setTimelineError(null);

    try {
      const nextStatus = await runStatusAction(api, action, status);
      setStatuses((current) => replaceStatus(current, nextStatus));
    } catch (caughtError) {
      setTimelineError(caughtError instanceof Error ? caughtError.message : "Could not update post.");
    } finally {
      setActingStatusId(null);
    }
  }

  async function deleteStatus(status: MastodonStatus) {
    if (!api) {
      return false;
    }

    setTimelineError(null);

    try {
      await api.deleteStatus(status.id);
      setStatuses((current) => current.filter((item) => item.id !== status.id));
      return true;
    } catch (caughtError) {
      setTimelineError(caughtError instanceof Error ? caughtError.message : "Could not delete post.");
      return false;
    }
  }

  async function editStatus(status: MastodonStatus, values: ComposeValues) {
    if (!api) {
      return false;
    }

    setTimelineError(null);

    try {
      const updated = await api.updateStatus(status.id, {
        status: values.status,
        visibility: values.visibility,
        sensitive: values.sensitive,
        spoiler_text: values.spoilerText,
        media_ids: values.mediaIds,
      });
      setStatuses((current) => replaceStatus(current, updated));
      return true;
    } catch (caughtError) {
      setTimelineError(caughtError instanceof Error ? caughtError.message : "Could not edit post.");
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
      const createdStatus = await api.createStatus({
        status: values.status,
        visibility: values.visibility,
        sensitive: values.sensitive,
        spoiler_text: values.spoilerText,
        media_ids: values.mediaIds,
        in_reply_to_id: replyingTo.id,
      });
      setReplyingTo(null);
      if (activeTimeline === "home") {
        setStatuses((current) => [createdStatus, ...current]);
      }
    } catch (caughtError) {
      setReplyError(caughtError instanceof Error ? caughtError.message : "Could not post reply.");
    } finally {
      setIsReplying(false);
    }
  }

  function handleTimelineClick(event: MouseEvent<HTMLElement>) {
    const target = event.target;
    if (!(target instanceof Element)) {
      return;
    }
    const link = target.closest("a");
    const href = link?.getAttribute("href") ?? "";
    if (href.includes("#/statuses/")) {
      saveCurrentTimeline(window.scrollY);
    }
  }

  async function submitPost(values: ComposeValues) {
    if (!api) {
      return;
    }

    setIsPosting(true);
    setPublishError(null);

    try {
      const createdStatus = await api.createStatus({
        status: values.status,
        visibility: values.visibility,
        sensitive: values.sensitive,
        spoiler_text: values.spoilerText,
        media_ids: values.mediaIds,
      });
      if (activeTimeline === "home") {
        setStatuses((current) => [createdStatus, ...current]);
      }
    } catch (caughtError) {
      setPublishError(caughtError instanceof Error ? caughtError.message : "Could not publish post.");
    } finally {
      setIsPosting(false);
    }
  }

  return (
    <section className="mx-auto max-w-2xl space-y-5" onClickCapture={handleTimelineClick}>
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <Tabs value={activeTimeline} onValueChange={navigateTimeline} items={[...timelineTabs]} />
        <Button variant="outline" size="sm" onClick={() => void loadTimeline(activeTimeline)} disabled={isLoading}>
          {isLoading ? "Refreshing..." : "Refresh"}
        </Button>
      </div>

      <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
        <ComposeForm
          submitLabel="Publish"
          submittingLabel="Publishing..."
          placeholder="Write a post"
          isSubmitting={isPosting}
          error={publishError}
          onSubmit={submitPost}
          onUploadMedia={api?.uploadMedia}
          onDeleteMedia={api?.deleteMedia}
          onUpdateMedia={api?.updateMedia}
          searchKnownAccounts={searchKnownAccounts}
          compact
        />
      </div>

      <div className="rounded-lg border border-border bg-card p-5 shadow-sm">

        {replyingTo ? (
          <div className="mb-5">
            <ReplyComposer
              status={replyingTo}
              isSubmitting={isReplying}
              error={replyError}
              onCancel={() => setReplyingTo(null)}
              onSubmit={submitReply}
            />
          </div>
        ) : null}

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
          <>
            <StatusList
              statuses={statuses}
              currentAccountId={currentAccount?.id}
              emptyTitle="No posts"
              emptyDescription="Nothing to show here yet."
              onDelete={deleteStatus}
              onEdit={editStatus}
              actingStatusId={actingStatusId}
              onAction={runAction}
              onForward={setForwardingStatus}
              onReply={(status) => {
                setReplyingTo(status);
                setReplyError(null);
              }}
            />
            <div className="mt-5 flex justify-center">
              <Button variant="outline" onClick={() => void loadMore()} disabled={isLoadingMore || !hasMore}>
                {isLoadingMore ? "Loading..." : hasMore ? "Load more" : "No more posts"}
              </Button>
            </div>
          </>
        )}
      </div>

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

function routeToTimeline(route: string): TimelineTab {
  if (route === "/local") {
    return "local";
  }
  if (route === "/global") {
    return "global";
  }
  return "home";
}
