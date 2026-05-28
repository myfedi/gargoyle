import { EmptyState, FeaturePage, Panel } from "@/features/shared";

export function DeliveryPage() {
  return (
    <FeaturePage
      eyebrow="Operations"
      title="Delivery"
      description="A calm place to see whether outgoing activity reached other servers or needs attention."
      status="planned"
    >
      <Panel title="Delivery health" description="Retries, failures, and successful deliveries will be shown here when available.">
        <EmptyState
          title="No delivery issues"
          description="There is nothing to review right now. When delivery tracking is ready, failed sends and retry details will appear here."
        />
      </Panel>
    </FeaturePage>
  );
}
