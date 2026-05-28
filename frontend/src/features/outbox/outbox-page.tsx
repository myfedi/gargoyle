import { EmptyState, FeaturePage, Panel } from "@/features/shared";

export function OutboxPage() {
  return (
    <FeaturePage
      eyebrow="Federation"
      title="Outbox"
      description="Published activities and fanout belong here after Gargoyle exposes an authenticated outbox inspection endpoint."
      status="needs-api"
    >
      <Panel title="Published activities" description="Posting itself is available from Posts through POST /api/v1/statuses.">
        <EmptyState
          title="No outbox inspection API connected"
          description="This should eventually show local Create activities, recipients, and delivery state without exposing raw protocol details by default."
        />
      </Panel>
    </FeaturePage>
  );
}
