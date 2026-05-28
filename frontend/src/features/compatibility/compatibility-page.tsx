import { FeaturePage, Panel } from "@/features/shared";

const implemented = ["Sign-in", "Account check", "Publishing", "Home timeline", "Public timeline"];
const notImplemented = ["Follow management", "Inbox activity view", "Outbox activity view", "Delivery tracking"];

export function CompatibilityPage() {
  return (
    <FeaturePage eyebrow="Health" title="Compatibility" description="Current frontend coverage.">
      <div className="grid gap-6 xl:grid-cols-2">
        <Panel title="Implemented">
          <ul className="list-disc space-y-2 pl-5 text-sm text-muted-foreground">
            {implemented.map((item) => (
              <li key={item}>{item}</li>
            ))}
          </ul>
        </Panel>

        <Panel title="Not implemented">
          <ul className="list-disc space-y-2 pl-5 text-sm text-muted-foreground">
            {notImplemented.map((item) => (
              <li key={item}>{item}</li>
            ))}
          </ul>
        </Panel>
      </div>
    </FeaturePage>
  );
}
