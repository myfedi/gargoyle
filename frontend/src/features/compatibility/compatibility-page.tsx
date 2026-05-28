import { Badge } from "@/components/ui/badge";
import { FeaturePage, Panel } from "@/features/shared";

const checks = [
  { label: "Sign-in", state: "healthy" },
  { label: "Account access", state: "healthy" },
  { label: "Publishing", state: "healthy" },
  { label: "Home timeline", state: "healthy" },
  { label: "Public timeline", state: "healthy" },
  { label: "Follow management", state: "later" },
  { label: "Delivery tracking", state: "later" },
];

export function CompatibilityPage() {
  return (
    <FeaturePage
      eyebrow="Health"
      title="Compatibility"
      description="A plain-language view of what this console can do today and what still needs attention."
      status="ready"
    >
      <Panel title="Console readiness" description="Useful when you are testing this instance against other fediverse software.">
        <div className="divide-y divide-border">
          {checks.map((check) => (
            <div key={check.label} className="flex items-center justify-between gap-4 py-3 text-sm first:pt-0 last:pb-0">
              <span>{check.label}</span>
              <Badge variant={check.state === "healthy" ? "success" : "warning"}>
                {check.state === "healthy" ? "Ready" : "Later"}
              </Badge>
            </div>
          ))}
        </div>
      </Panel>
    </FeaturePage>
  );
}
