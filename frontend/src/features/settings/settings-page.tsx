import { FeaturePage, FieldRow, Panel } from "@/features/shared";
import { getApiBaseUrl, getOAuthConfig } from "@/lib/config";

export function SettingsPage() {
  const oauth = getOAuthConfig();

  return (
    <FeaturePage
      eyebrow="Instance"
      title="Settings"
      description="Runtime configuration for the local UI. Secrets are intentionally absent because this browser client uses OAuth PKCE."
      status="ready"
    >
      <div className="grid gap-6 xl:grid-cols-2">
        <Panel title="API" description="Requests are same-origin in development through the Vite proxy unless an absolute base URL is configured.">
          <dl>
            <FieldRow label="API base URL" value={getApiBaseUrl() || "Same origin"} />
          </dl>
        </Panel>

        <Panel title="OAuth" description="Public client configuration only. Do not add client_secret to this app.">
          {oauth ? (
            <dl>
              <FieldRow label="Client ID" value={<code className="text-xs">{oauth.clientId}</code>} />
              <FieldRow label="Authorize URL" value={oauth.authorizationEndpoint} />
              <FieldRow label="Token URL" value={oauth.tokenEndpoint} />
              <FieldRow label="Redirect URI" value={oauth.redirectUri} />
              <FieldRow label="Scopes" value={oauth.scopes.join(" ")} />
            </dl>
          ) : (
            <p className="text-sm text-muted-foreground">OAuth client ID is not configured.</p>
          )}
        </Panel>
      </div>
    </FeaturePage>
  );
}
