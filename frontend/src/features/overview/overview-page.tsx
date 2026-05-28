import { useEffect, useMemo, useState } from "react";
import { Activity, CircleCheck, Clock, ShieldCheck } from "lucide-react";

import { useAuth } from "@/app/auth-context";
import { Badge } from "@/components/ui/badge";
import { EmptyState, FeaturePage, FieldRow, Panel } from "@/features/shared";
import { createMastodonApi } from "@/lib/mastodon-api";
import type { MastodonInstance } from "@/types/mastodon";

const readinessItems = [
  { label: "OAuth", value: "PKCE browser flow", icon: ShieldCheck, variant: "success" as const },
  { label: "Account", value: "verify_credentials wired", icon: CircleCheck, variant: "success" as const },
  { label: "Timelines", value: "Home and public API ready", icon: Activity, variant: "secondary" as const },
  { label: "Delivery", value: "Awaiting queue API", icon: Clock, variant: "warning" as const },
];

export function OverviewPage() {
  const { session } = useAuth();
  const [instance, setInstance] = useState<MastodonInstance | null>(null);
  const [error, setError] = useState<string | null>(null);
  const api = useMemo(() => (session?.accessToken ? createMastodonApi(session.accessToken) : null), [session?.accessToken]);

  useEffect(() => {
    if (!api) {
      return;
    }

    let cancelled = false;
    api
      .instance()
      .then((nextInstance) => {
        if (!cancelled) {
          setInstance(nextInstance);
          setError(null);
        }
      })
      .catch((caughtError: unknown) => {
        if (!cancelled) {
          setError(caughtError instanceof Error ? caughtError.message : "Could not load instance details.");
        }
      });

    return () => {
      cancelled = true;
    };
  }, [api]);

  return (
    <FeaturePage
      eyebrow="Instance console"
      title="Your Gargoyle surface"
      description="A small-instance control room for publishing, authorization, and federation health. The UI shows real API state where endpoints exist, and stays honest where backend support is pending."
      status="ready"
      primaryAction="Write a post"
    >
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        {readinessItems.map((item) => {
          const Icon = item.icon;
          return (
            <Panel key={item.label} title={item.label} className="min-h-36">
              <div className="flex items-center justify-between gap-3">
                <div className="space-y-2">
                  <p className="text-sm text-muted-foreground">{item.value}</p>
                  <Badge variant={item.variant}>Tracked</Badge>
                </div>
                <span className="rounded-md bg-secondary p-2 text-secondary-foreground">
                  <Icon className="size-4" aria-hidden="true" />
                </span>
              </div>
            </Panel>
          );
        })}
      </div>

      <Panel title="Instance" description="Loaded from GET /api/v1/instance.">
        {error ? (
          <p className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive" role="alert">
            {error}
          </p>
        ) : instance ? (
          <dl>
            <FieldRow label="Title" value={instance.title} />
            <FieldRow label="Domain" value={instance.uri ?? instance.domain ?? "Unknown"} />
            <FieldRow label="Version" value={instance.version} />
            <FieldRow label="Description" value={instance.short_description ?? instance.description ?? "No description published."} />
          </dl>
        ) : (
          <EmptyState title="Loading instance" description="Fetching Gargoyle instance metadata." />
        )}
      </Panel>
    </FeaturePage>
  );
}
