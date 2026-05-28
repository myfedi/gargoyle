import { EmptyState, FeaturePage, Panel } from "@/features/shared";

export function DeliveryPage() {
  return (
    <FeaturePage eyebrow="Operations" title="Delivery" description="Delivery attempts and failures.">
      <Panel title="Not implemented">
        <EmptyState
          title="Delivery tracking is not implemented"
          description="Needed: queue, attempts, next retry, delivered time, last error, retry, and cancel."
        />
      </Panel>
    </FeaturePage>
  );
}
