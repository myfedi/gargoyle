import type React from "react";
import { useCallback, useEffect, useMemo, useState } from "react";

import { useAuth } from "@/app/auth-context";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { EmptyState, FeaturePage, Panel } from "@/features/shared";
import { ApiError } from "@/lib/api";
import { createMastodonApi } from "@/lib/mastodon-api";
import { formatDateTime } from "@/lib/text";
import type { RelaySubscription } from "@/types/mastodon";

export function RelaysPage() {
  const { session } = useAuth();
  const [relays, setRelays] = useState<RelaySubscription[]>([]);
  const [actorURI, setActorURI] = useState("");
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [busyRelay, setBusyRelay] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const api = useMemo(() => (session?.accessToken ? createMastodonApi(session.accessToken) : null), [session?.accessToken]);

  const loadRelays = useCallback(async () => {
    if (!api) return;
    setIsLoading(true);
    setError(null);
    try {
      setRelays(await api.relays());
    } catch (caughtError) {
      if (caughtError instanceof ApiError && caughtError.status === 401) {
        setError("Admin access is required.");
      } else {
        setError(caughtError instanceof Error ? caughtError.message : "Could not load relays.");
      }
    } finally {
      setIsLoading(false);
    }
  }, [api]);

  useEffect(() => {
    void loadRelays();
  }, [loadRelays]);

  async function submitRelay(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!api || !actorURI.trim()) return;
    setIsSaving(true);
    setError(null);
    setNotice(null);
    try {
      const relay = await api.createRelay(actorURI.trim());
      setRelays((current) => [relay, ...current.filter((item) => item.actor_uri !== relay.actor_uri)].sort((a, b) => a.actor_uri.localeCompare(b.actor_uri)));
      setActorURI("");
      setNotice(`Sent relay follow to ${relay.actor_uri}. It will become accepted when the relay replies.`);
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not add relay.");
    } finally {
      setIsSaving(false);
    }
  }

  async function disableRelay(relay: RelaySubscription) {
    if (!api) return;
    setBusyRelay(relay.actor_uri);
    setError(null);
    setNotice(null);
    try {
      const updated = await api.disableRelay(relay.id);
      setRelays((current) => current.map((item) => (item.actor_uri === updated.actor_uri ? updated : item)));
      setNotice(`Disabled ${relay.actor_uri} and queued an Undo Follow.`);
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not disable relay.");
    } finally {
      setBusyRelay(null);
    }
  }

  async function deleteRelay(relay: RelaySubscription) {
    if (!api) return;
    setBusyRelay(relay.actor_uri);
    setError(null);
    setNotice(null);
    try {
      await api.deleteRelay(relay.id);
      setRelays((current) => current.filter((item) => item.actor_uri !== relay.actor_uri));
      setNotice(`Removed ${relay.actor_uri}.`);
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not remove relay.");
    } finally {
      setBusyRelay(null);
    }
  }

  return (
    <FeaturePage eyebrow="Admin" title="Federation relays" description="Subscribe this instance to opt-in ActivityPub relays, then disable them again when they are too noisy.">
      <Panel title="Add a relay" description="Relays receive public posts only after you explicitly follow them. Pending relays are not used for fan-out until they Accept.">
        <form className="space-y-4" onSubmit={(event) => void submitRelay(event)}>
          <div className="grid gap-2">
            <label className="text-sm font-medium" htmlFor="relay-actor">Relay actor URL</label>
            <Input id="relay-actor" value={actorURI} placeholder="https://relay.example/actor" disabled={isSaving} onChange={(event) => setActorURI(event.target.value)} />
          </div>
          <Button type="submit" disabled={isSaving || !actorURI.trim()}>{isSaving ? "Adding..." : "Add relay"}</Button>
        </form>
      </Panel>

      <Panel title="Configured relays">
        <div className="mb-4 flex justify-end">
          <Button variant="outline" size="sm" disabled={isLoading} onClick={() => void loadRelays()}>Refresh</Button>
        </div>
        {error ? <p className="mb-4 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive" role="alert">{error}</p> : null}
        {notice ? <p className="mb-4 rounded-md border border-border bg-secondary px-3 py-2 text-sm text-secondary-foreground" role="status">{notice}</p> : null}
        {isLoading ? (
          <div className="space-y-3">{[0, 1, 2].map((item) => <div key={item} className="h-20 animate-pulse rounded-md bg-secondary" />)}</div>
        ) : relays.length === 0 ? (
          <EmptyState title="No relays configured" description="Add a relay actor URL to start a subscription request." />
        ) : (
          <div className="divide-y divide-border">
            {relays.map((relay) => {
              const isBusy = busyRelay === relay.actor_uri;
              const active = relay.status === "accepted";
              return (
                <article key={relay.id} className="flex flex-col gap-4 py-4 first:pt-0 last:pb-0 lg:flex-row lg:items-start lg:justify-between">
                  <div className="min-w-0 space-y-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <h2 className="break-all font-semibold">{relay.actor_uri}</h2>
                      <span className="rounded-full bg-secondary px-2 py-0.5 text-xs capitalize text-secondary-foreground">{relay.status}</span>
                      {active ? <span className="rounded-full bg-secondary px-2 py-0.5 text-xs text-secondary-foreground">Outbound enabled</span> : null}
                    </div>
                    <p className="break-all text-sm text-muted-foreground">Inbox: {relay.inbox_uri}</p>
                    <p className="text-xs text-muted-foreground">Updated {formatDateTime(relay.updated_at)}</p>
                    {relay.accepted_at ? <p className="text-xs text-muted-foreground">Accepted {formatDateTime(relay.accepted_at)}</p> : null}
                    {relay.last_error ? <p className="text-sm text-destructive">{relay.last_error}</p> : null}
                  </div>
                  <div className="flex flex-wrap gap-2">
                    <Button variant="outline" size="sm" disabled={isBusy || relay.status === "disabled"} onClick={() => void disableRelay(relay)}>{isBusy ? "Working..." : "Disable"}</Button>
                    <Button variant="destructive" size="sm" disabled={isBusy} onClick={() => void deleteRelay(relay)}>Remove</Button>
                  </div>
                </article>
              );
            })}
          </div>
        )}
      </Panel>
    </FeaturePage>
  );
}
