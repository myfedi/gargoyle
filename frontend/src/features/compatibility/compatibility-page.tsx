import { Badge } from "@/components/ui/badge";
import { FeaturePage, Panel } from "@/features/shared";

const checks = [
  { label: "POST /api/v1/apps", state: "available" },
  { label: "OAuth Authorization Code + PKCE", state: "available" },
  { label: "GET /api/v1/accounts/verify_credentials", state: "available" },
  { label: "POST /api/v1/statuses", state: "available" },
  { label: "GET /api/v1/timelines/home", state: "available" },
  { label: "GET /api/v1/timelines/public", state: "available" },
  { label: "Followers/following API", state: "pending" },
  { label: "Delivery queue API", state: "pending" },
];

export function CompatibilityPage() {
  return (
    <FeaturePage
      eyebrow="Interoperability"
      title="Compatibility"
      description="A concise view of the Mastodon-compatible surface this frontend expects from Gargoyle."
      status="ready"
    >
      <Panel title="Mastodon API readiness" description="Frontend wiring follows this contract and avoids client secrets.">
        <div className="divide-y divide-border">
          {checks.map((check) => (
            <div key={check.label} className="flex items-center justify-between gap-4 py-3 text-sm first:pt-0 last:pb-0">
              <span>{check.label}</span>
              <Badge variant={check.state === "available" ? "success" : "warning"}>
                {check.state === "available" ? "Available" : "Pending"}
              </Badge>
            </div>
          ))}
        </div>
      </Panel>
    </FeaturePage>
  );
}
