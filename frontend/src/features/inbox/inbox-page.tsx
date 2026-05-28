import { EmptyState, FeaturePage, Panel } from "@/features/shared";

export function InboxPage() {
  return (
    <FeaturePage eyebrow="Federation" title="Inbox" description="Incoming federation activity.">
      <Panel title="Not implemented">
        <EmptyState
          title="Inbox view is not implemented"
          description="Needed: incoming follows, mentions, updates, deletes, accepts, rejects, filters, and inspect view."
        />
      </Panel>
    </FeaturePage>
  );
}
