import { useCallback, useEffect, useMemo, useState } from "react";
import { ArrowRight, ExternalLink, Globe2, Loader2, Users } from "lucide-react";

import { useAuth } from "@/app/auth-context";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Panel } from "@/features/shared";
import { createMastodonApi } from "@/lib/mastodon-api";
import { accountHref } from "@/lib/routes";
import { htmlToPlainText } from "@/lib/text";
import type { MastodonAccount, MastodonRelationship } from "@/types/mastodon";

type ExternalInteractionPageProps = {
  route: string;
};

export function ExternalInteractionPage({ route }: ExternalInteractionPageProps) {
  const { session } = useAuth();
  const uri = useMemo(() => new URLSearchParams(route.split("?")[1] ?? "").get("uri") ?? "", [route]);
  const api = useMemo(() => (session?.accessToken ? createMastodonApi(session.accessToken) : null), [session?.accessToken]);
  const [account, setAccount] = useState<MastodonAccount | null>(null);
  const [relationship, setRelationship] = useState<MastodonRelationship | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isFollowing, setIsFollowing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const resolve = useCallback(async () => {
    if (!api) return;
    setIsLoading(true);
    setError(null);
    setAccount(null);
    setRelationship(null);

    try {
      const result = await api.externalInteraction(uri);
      if (result.type !== "account" || !result.account) {
        throw new Error("Gargoyle found this remote resource, but cannot interact with this type yet.");
      }
      setAccount(result.account);
      const [nextRelationship] = await api.relationships([result.account.id]);
      setRelationship(nextRelationship ?? null);
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not resolve this remote content.");
    } finally {
      setIsLoading(false);
    }
  }, [api, uri]);

  useEffect(() => {
    void resolve();
  }, [resolve]);

  async function followAccount() {
    if (!api || !account) return;
    setIsFollowing(true);
    setError(null);
    try {
      await api.followAccount(account.id);
      const [nextRelationship] = await api.relationships([account.id]);
      setRelationship(nextRelationship ?? null);
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not follow this remote profile.");
    } finally {
      setIsFollowing(false);
    }
  }

  if (!uri) {
    return (
      <ExternalInteractionShell title="No remote link provided" uri="">
        <p className="text-sm leading-6 text-muted-foreground">Open this page from a Fediverse remote interaction link, or paste a remote profile URL into search.</p>
        <Button asChild className="mt-5">
          <a href="/#/">Go to timeline</a>
        </Button>
      </ExternalInteractionShell>
    );
  }

  if (isLoading) {
    return (
      <ExternalInteractionShell title="Finding this on the Fediverse" uri={uri}>
        <div className="flex items-center gap-3 rounded-lg border border-border bg-card px-4 py-3 text-sm text-muted-foreground shadow-sm">
          <Loader2 className="size-4 animate-spin text-primary" aria-hidden="true" />
          Resolving the remote profile with your local account identity.
        </div>
      </ExternalInteractionShell>
    );
  }

  if (error || !account) {
    return (
      <ExternalInteractionShell title="This remote link is not ready here yet" uri={uri}>
        <Panel title="Could not resolve this resource" description={error ?? "Gargoyle could not turn this remote link into an interaction target."}>
          <div className="flex flex-wrap gap-3">
            <Button onClick={() => void resolve()}>Try again</Button>
            <Button asChild variant="outline">
              <a href={uri} rel="noreferrer" target="_blank">
                Open original <ExternalLink className="ml-2 size-4" aria-hidden="true" />
              </a>
            </Button>
          </div>
        </Panel>
      </ExternalInteractionShell>
    );
  }

  const isAlreadyFollowing = Boolean(relationship?.following);
  const isRequested = Boolean(relationship?.requested);
  const profileLabel = account.group ? "Remote community" : "Remote profile";
  const note = htmlToPlainText(account.note ?? "").trim();

  return (
    <ExternalInteractionShell title="Interact from Gargoyle" uri={uri}>
      <section className="overflow-hidden rounded-xl border border-border bg-card shadow-sm" aria-labelledby="remote-account-title">
        {account.header ? <img src={account.header} alt="" className="h-32 w-full object-cover" /> : <div className="h-20 bg-secondary" />}
        <div className="p-5 md:p-6">
          <div className="flex flex-col gap-5 sm:flex-row sm:items-start sm:justify-between">
            <div className="flex min-w-0 gap-4">
              <div className="size-16 shrink-0 overflow-hidden rounded-xl border border-border bg-background">
                {account.avatar ? <img src={account.avatar} alt="" className="size-full object-cover" /> : <Users className="m-5 size-6 text-muted-foreground" aria-hidden="true" />}
              </div>
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <Badge variant="secondary">{profileLabel}</Badge>
                  {account.locked ? <Badge variant="outline">Approval required</Badge> : null}
                </div>
                <h1 id="remote-account-title" className="mt-3 text-2xl font-semibold tracking-tight">
                  {account.display_name || account.username}
                </h1>
                <p className="mt-1 truncate text-sm text-muted-foreground">@{account.acct}</p>
              </div>
            </div>
            <div className="flex flex-wrap gap-2 sm:justify-end">
              <Button asChild variant="outline">
                <a href={accountHref(account.id)}>
                  Open profile <ArrowRight className="ml-2 size-4" aria-hidden="true" />
                </a>
              </Button>
              <Button onClick={() => void followAccount()} disabled={isFollowing || isAlreadyFollowing || isRequested}>
                {isAlreadyFollowing ? "Following" : isRequested ? "Requested" : isFollowing ? "Following..." : account.group ? "Follow community" : "Follow profile"}
              </Button>
            </div>
          </div>

          {note ? <p className="mt-5 max-w-2xl text-sm leading-6 text-muted-foreground">{note}</p> : null}

          <div className="mt-6 flex flex-wrap gap-3 text-sm text-muted-foreground">
            <span>{account.followers_count ?? 0} followers</span>
            <span>{account.statuses_count ?? 0} posts</span>
            {account.url ? (
              <a href={account.url} rel="noreferrer" target="_blank" className="inline-flex items-center gap-1 text-primary hover:underline">
                <Globe2 className="size-4" aria-hidden="true" /> Original page
              </a>
            ) : null}
          </div>
        </div>
      </section>

      {error ? <p className="text-sm text-destructive" role="alert">{error}</p> : null}
    </ExternalInteractionShell>
  );
}

function ExternalInteractionShell({ title, uri, children }: { title: string; uri: string; children: React.ReactNode }) {
  return (
    <section className="space-y-6">
      <div className="max-w-3xl space-y-3 border-b border-border pb-6">
        <p className="text-sm font-medium text-muted-foreground">Remote interaction</p>
        <h1 className="text-3xl font-semibold tracking-tight">{title}</h1>
        {uri ? <p className="break-all text-sm leading-6 text-muted-foreground">{uri}</p> : null}
      </div>
      {children}
    </section>
  );
}
