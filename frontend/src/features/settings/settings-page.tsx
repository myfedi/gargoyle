import { FeaturePage, FieldRow, Panel } from "@/features/shared";
import { getApiBaseUrl, getOAuthConfig } from "@/lib/config";

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
      </div>
    </FeaturePage>
  );
}
