import { EmptyState, FeaturePage, Panel } from "@/features/shared";

export function OutboxPage() {
  return (
    <FeaturePage eyebrow="Federation" title="Outbox" description="Sent federation activity.">
      <Panel title="Not implemented">
        <EmptyState
          title="Outbox view is not implemented"
          description="Needed: sent activities, recipients, audience, and delivery state."
        />
      </Panel>
    </FeaturePage>
  );
}
