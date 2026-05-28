import { EmptyState, FeaturePage, Panel } from "@/features/shared";

export function InboxPage() {
  return (
    <FeaturePage
      eyebrow="Federation"
      title="Inbox"
      description="Incoming federation activity, summarized for people instead of protocol spelunking."
      status="needs-api"
    >
      <Panel title="Incoming activity" description="Follows, mentions, updates, and other arrivals will be collected here.">
        <EmptyState
          title="No incoming activity yet"
          description="When other servers interact with this instance, this page will help you understand what arrived and whether it needs attention."
        />
      </Panel>
    </FeaturePage>
  );
}
