import { EmptyState, FeaturePage, Panel } from "@/features/shared";

export function OutboxPage() {
  return (
    <FeaturePage
      eyebrow="Federation"
      title="Outbox"
      description="A readable record of what this instance has sent out to the fediverse."
      status="needs-api"
    >
      <Panel title="Sent activity" description="Published notes and follow-related activity will appear here.">
        <EmptyState
          title="No sent activity yet"
          description="Once the outbox view is ready, you will be able to confirm what left this instance and where it was headed."
        />
      </Panel>
    </FeaturePage>
  );
}
