import { EmptyState, FeaturePage, Panel } from "@/features/shared";

export function DeliveryPage() {
  return (
    <FeaturePage
      eyebrow="Operations"
      title="Delivery"
      description="Delivery attempts, retries, and remote inbox failures should be visible once Gargoyle has a persistent queue API."
      status="planned"
    >
      <Panel title="Queue" description="Designed for a future deliveries endpoint with attempts, next retry time, and last error.">
        <EmptyState
          title="Delivery queue not exposed yet"
          description="Keep this page operational and boring: failed inbox, attempts, next retry, last error, and delivered time."
        />
      </Panel>
    </FeaturePage>
  );
}
