import type React from "react";
import { useCallback, useEffect, useMemo, useState } from "react";

import { useAuth } from "@/app/auth-context";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { EmptyState, FeaturePage, Panel } from "@/features/shared";
import { ApiError } from "@/lib/api";
import { createMastodonApi } from "@/lib/mastodon-api";
import { formatDateTime } from "@/lib/text";
import type { DomainBlock } from "@/types/mastodon";

export function DomainModerationPage() {
  const { session } = useAuth();
  const [blocks, setBlocks] = useState<DomainBlock[]>([]);
  const [domain, setDomain] = useState("");
  const [publicComment, setPublicComment] = useState("");
  const [privateComment, setPrivateComment] = useState("");
  const [rejectMedia, setRejectMedia] = useState(true);
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [busyDomain, setBusyDomain] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const api = useMemo(() => (session?.accessToken ? createMastodonApi(session.accessToken) : null), [session?.accessToken]);

  const loadBlocks = useCallback(async () => {
    if (!api) return;
    setIsLoading(true);
    setError(null);
    try {
      setBlocks(await api.domainBlocks());
    } catch (caughtError) {
      if (caughtError instanceof ApiError && caughtError.status === 401) {
        setError("Admin access is required.");
      } else {
        setError(caughtError instanceof Error ? caughtError.message : "Could not load domain blocks.");
      }
    } finally {
      setIsLoading(false);
    }
  }, [api]);

  useEffect(() => {
    void loadBlocks();
  }, [loadBlocks]);

  async function submitBlock(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!api || !domain.trim()) return;
    setIsSaving(true);
    setError(null);
    setNotice(null);
    try {
      const block = await api.createDomainBlock({ domain: domain.trim(), public_comment: publicComment.trim(), private_comment: privateComment.trim(), reject_media: rejectMedia });
      setBlocks((current) => [block, ...current.filter((item) => item.domain !== block.domain)].sort((a, b) => a.domain.localeCompare(b.domain)));
      setDomain("");
      setPublicComment("");
      setPrivateComment("");
      setRejectMedia(true);
      setNotice(`Blocked ${block.domain}.`);
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not block domain.");
    } finally {
      setIsSaving(false);
    }
  }

  async function removeBlock(block: DomainBlock) {
    if (!api) return;
    setBusyDomain(block.domain);
    setError(null);
    setNotice(null);
    try {
      await api.deleteDomainBlock(block.domain);
      setBlocks((current) => current.filter((item) => item.domain !== block.domain));
      setNotice(`Removed block for ${block.domain}.`);
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not remove domain block.");
    } finally {
      setBusyDomain(null);
    }
  }

  async function purgeDomain(block: DomainBlock) {
    if (!api) return;
    setBusyDomain(block.domain);
    setError(null);
    setNotice(null);
    try {
      const job = await api.purgeDomain(block.domain);
      setNotice(`Queued ${job.kind} job for ${block.domain}.`);
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not queue purge.");
    } finally {
      setBusyDomain(null);
    }
  }

  return (
    <FeaturePage eyebrow="Admin" title="Domain moderation" description="Suspend remote servers and remove cached content when needed.">
      <Panel title="Block a domain" description="Suspended domains are refused during federation, hidden from client views, and skipped by delivery workers.">
        <form className="space-y-4" onSubmit={(event) => void submitBlock(event)}>
          <div className="grid gap-2">
            <label className="text-sm font-medium" htmlFor="domain-block-domain">Domain</label>
            <Input id="domain-block-domain" value={domain} placeholder="bad.example" disabled={isSaving} onChange={(event) => setDomain(event.target.value)} />
          </div>
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="grid gap-2">
              <label className="text-sm font-medium" htmlFor="domain-block-public">Public comment</label>
              <Textarea id="domain-block-public" value={publicComment} disabled={isSaving} onChange={(event) => setPublicComment(event.target.value)} />
            </div>
            <div className="grid gap-2">
              <label className="text-sm font-medium" htmlFor="domain-block-private">Private note</label>
              <Textarea id="domain-block-private" value={privateComment} disabled={isSaving} onChange={(event) => setPrivateComment(event.target.value)} />
            </div>
          </div>
          <label className="flex items-center gap-2 text-sm">
            <input className="size-4 accent-primary" type="checkbox" checked={rejectMedia} disabled={isSaving} onChange={(event) => setRejectMedia(event.target.checked)} />
            Reject media from this domain
          </label>
          <Button type="submit" disabled={isSaving || !domain.trim()}>{isSaving ? "Blocking..." : "Block domain"}</Button>
        </form>
      </Panel>

      <Panel title="Blocked domains">
        <div className="mb-4 flex justify-end">
          <Button variant="outline" size="sm" disabled={isLoading} onClick={() => void loadBlocks()}>Refresh</Button>
        </div>
        {error ? <p className="mb-4 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive" role="alert">{error}</p> : null}
        {notice ? <p className="mb-4 rounded-md border border-border bg-secondary px-3 py-2 text-sm text-secondary-foreground" role="status">{notice}</p> : null}
        {isLoading ? (
          <div className="space-y-3">{[0, 1, 2].map((item) => <div key={item} className="h-20 animate-pulse rounded-md bg-secondary" />)}</div>
        ) : blocks.length === 0 ? (
          <EmptyState title="No blocked domains" description="Suspended domains will appear here." />
        ) : (
          <div className="divide-y divide-border">
            {blocks.map((block) => {
              const isBusy = busyDomain === block.domain;
              return (
                <article key={block.id} className="flex flex-col gap-4 py-4 first:pt-0 last:pb-0 lg:flex-row lg:items-start lg:justify-between">
                  <div className="min-w-0 space-y-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <h2 className="font-semibold">{block.domain}</h2>
                      <span className="rounded-full bg-secondary px-2 py-0.5 text-xs capitalize text-secondary-foreground">{block.severity}</span>
                      {block.reject_media ? <span className="rounded-full bg-secondary px-2 py-0.5 text-xs text-secondary-foreground">Reject media</span> : null}
                    </div>
                    {block.public_comment ? <p className="text-sm text-muted-foreground">{block.public_comment}</p> : null}
                    {block.private_comment ? <p className="text-sm text-muted-foreground">Private: {block.private_comment}</p> : null}
                    <p className="text-xs text-muted-foreground">Updated {formatDateTime(block.updated_at)}</p>
                  </div>
                  <div className="flex flex-wrap gap-2">
                    <Button variant="outline" size="sm" disabled={isBusy} onClick={() => void purgeDomain(block)}>{isBusy ? "Working..." : "Remove cached content"}</Button>
                    <Button variant="destructive" size="sm" disabled={isBusy} onClick={() => void removeBlock(block)}>Unblock</Button>
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
