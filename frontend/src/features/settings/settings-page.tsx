import { useEffect, useMemo, useState } from "react";

import { Button } from "@/components/ui/button";
import { useAuth } from "@/app/auth-context";
import { FeaturePage, FieldRow, Panel } from "@/features/shared";
import { ApiError } from "@/lib/api";
import { getApiBaseUrl, getOAuthConfig, getVapidPublicKey } from "@/lib/config";
import { createMastodonApi } from "@/lib/mastodon-api";
import { alertsFromSubscription, defaultPushAlerts, ensurePushSubscription, pushSupport, subscriptionToServerPayload, unsubscribeBrowserPush, type PushAlertSettings } from "@/lib/push";
import type { MastodonPushSubscription } from "@/types/mastodon";

const pushAlertLabels: Array<[keyof PushAlertSettings, string]> = [
  ["mention", "Mentions and replies"],
  ["follow", "New followers"],
  ["follow_request", "Follow requests"],
  ["favourite", "Favourites"],
  ["reblog", "Boosts"],
  ["poll", "Poll results"],
  ["status", "New posts from people you follow"],
  ["update", "Edited posts"],
];

export function SettingsPage() {
  const oauth = getOAuthConfig();

  return (
    <FeaturePage eyebrow="Instance" title="Settings" description="Connection settings.">
      <div className="grid gap-6 xl:grid-cols-2">
        <Panel title="Server">
          <dl>
            <FieldRow label="Base URL" value={getApiBaseUrl() || "Same origin"} />
          </dl>
        </Panel>

        <Panel title="Sign-in">
          {oauth ? (
            <dl>
              <FieldRow label="Client" value={<code className="text-xs">{oauth.clientId}</code>} />
              <FieldRow label="Authorize URL" value={oauth.authorizationEndpoint} />
              <FieldRow label="Redirect URI" value={oauth.redirectUri} />
              <FieldRow label="Permissions" value={oauth.scopes.join(" ")} />
            </dl>
          ) : (
            <p className="text-sm text-muted-foreground">Sign-in is not configured.</p>
          )}
        </Panel>

        <PushNotificationsPanel />

        <Panel title="Admin" description="Moderation and federation tools are available to instance admins.">
          <div className="flex flex-wrap gap-2">
            <Button asChild variant="outline">
              <a href="/#/admin/moderation/domains">Domain moderation</a>
            </Button>
            <Button asChild variant="outline">
              <a href="/#/admin/federation/relays">Federation relays</a>
            </Button>
          </div>
        </Panel>
      </div>
    </FeaturePage>
  );
}

function PushNotificationsPanel() {
  const { session } = useAuth();
  const support = useMemo(() => pushSupport(), []);
  const api = useMemo(() => (session?.accessToken ? createMastodonApi(session.accessToken) : null), [session?.accessToken]);
  const [serverSubscription, setServerSubscription] = useState<MastodonPushSubscription | null>(null);
  const [alerts, setAlerts] = useState<PushAlertSettings>(defaultPushAlerts);
  const [status, setStatus] = useState("Checking push subscription...");
  const [isSaving, setIsSaving] = useState(false);

  const hasPushScope = session?.scope?.split(/\s+/).includes("push") ?? false;
  const vapidPublicKey = getVapidPublicKey() || serverSubscription?.server_key || "";

  useEffect(() => {
    let cancelled = false;
    async function loadPushSubscription() {
      if (!api) {
        setStatus("Sign in to manage push notifications.");
        return;
      }
      if (!hasPushScope) {
        setStatus("Sign in again to grant the push permission.");
        return;
      }
      try {
        const subscription = await api.pushSubscription();
        if (cancelled) return;
        setServerSubscription(subscription);
        setAlerts(alertsFromSubscription(subscription));
        setStatus("Push notifications are enabled on this device.");
      } catch (error) {
        if (cancelled) return;
        if (error instanceof ApiError && error.status === 404) {
          setStatus("Push notifications are off on this device.");
          return;
        }
        setStatus(error instanceof Error ? error.message : "Could not check push notifications.");
      }
    }
    void loadPushSubscription();
    return () => {
      cancelled = true;
    };
  }, [api, hasPushScope]);

  async function enablePush() {
    if (!api) return;
    setIsSaving(true);
    setStatus("Waiting for browser permission...");
    try {
      const browserSubscription = await ensurePushSubscription(vapidPublicKey);
      const saved = await api.createPushSubscription(subscriptionToServerPayload(browserSubscription, alerts));
      setServerSubscription(saved);
      setAlerts(alertsFromSubscription(saved));
      setStatus("Push notifications are enabled on this device.");
    } catch (error) {
      setStatus(error instanceof Error ? error.message : "Could not enable push notifications.");
    } finally {
      setIsSaving(false);
    }
  }

  async function saveAlerts(nextAlerts: PushAlertSettings) {
    if (!api || !serverSubscription) return;
    setAlerts(nextAlerts);
    setIsSaving(true);
    try {
      const saved = await api.updatePushSubscription({ data: { alerts: nextAlerts, policy: serverSubscription.policy || "all" } });
      setServerSubscription(saved);
      setAlerts(alertsFromSubscription(saved));
      setStatus("Push notification choices saved.");
    } catch (error) {
      setStatus(error instanceof Error ? error.message : "Could not save push notification choices.");
    } finally {
      setIsSaving(false);
    }
  }

  async function disablePush() {
    if (!api) return;
    setIsSaving(true);
    try {
      await api.deletePushSubscription();
      await unsubscribeBrowserPush();
      setServerSubscription(null);
      setStatus("Push notifications are off on this device.");
    } catch (error) {
      setStatus(error instanceof Error ? error.message : "Could not disable push notifications.");
    } finally {
      setIsSaving(false);
    }
  }

  return (
    <Panel title="Push notifications" description="Get mentions, follow requests, boosts, favourites, polls, and direct-message mentions on this device.">
      <div className="space-y-4">
        <p className="text-sm text-muted-foreground">{support.supported ? status : support.reason}</p>
        {!hasPushScope ? <p className="text-sm text-muted-foreground">Your current session does not include the push scope. Sign out and sign in again.</p> : null}
        {!vapidPublicKey ? <p className="text-sm text-muted-foreground">This server has not exposed a VAPID public key to the frontend.</p> : null}

        <div className="flex flex-wrap gap-2">
          <Button onClick={() => void enablePush()} disabled={!support.supported || !api || !hasPushScope || !vapidPublicKey || isSaving}>
            {serverSubscription ? "Refresh subscription" : "Enable on this device"}
          </Button>
          <Button variant="outline" onClick={() => void disablePush()} disabled={!api || !serverSubscription || isSaving}>
            Disable
          </Button>
        </div>

        <div className="space-y-2">
          {pushAlertLabels.map(([key, label]) => (
            <label key={key} className="flex items-center justify-between gap-4 rounded-md border border-border px-3 py-2 text-sm">
              <span>{label}</span>
              <input
                type="checkbox"
                className="h-4 w-4 accent-primary"
                checked={alerts[key]}
                disabled={!serverSubscription || isSaving}
                onChange={(event) => void saveAlerts({ ...alerts, [key]: event.target.checked })}
              />
            </label>
          ))}
        </div>
      </div>
    </Panel>
  );
}
