import { FeaturePage, FieldRow, Panel } from "@/features/shared";
import { getApiBaseUrl, getOAuthConfig } from "@/lib/config";

export function SettingsPage() {
  const oauth = getOAuthConfig();

  return (
    <FeaturePage
      eyebrow="Instance"
      title="Settings"
      description="Review how this console connects to your Gargoyle instance."
      status="ready"
    >
      <div className="grid gap-6 xl:grid-cols-2">
        <Panel title="Connection" description="Where this console sends requests while you use it.">
          <dl>
            <FieldRow label="Server" value={getApiBaseUrl() || "Same origin"} />
          </dl>
        </Panel>

        <Panel title="Authorization" description="The sign-in details this console uses for Gargoyle.">
          {oauth ? (
            <dl>
              <FieldRow label="Client" value={<code className="text-xs">{oauth.clientId}</code>} />
              <FieldRow label="Sign-in page" value={oauth.authorizationEndpoint} />
              <FieldRow label="Return address" value={oauth.redirectUri} />
              <FieldRow label="Permissions" value={oauth.scopes.join(" ")} />
            </dl>
          ) : (
            <p className="text-sm text-muted-foreground">Sign-in is not configured yet.</p>
          )}
        </Panel>
      </div>
    </FeaturePage>
  );
}
