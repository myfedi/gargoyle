import { EmptyState, FeaturePage, Panel } from "@/features/shared";

export function DirectMessagesPage() {
  return (
    <FeaturePage eyebrow="Messages" title="Direct messages" description="Private conversations.">
      <Panel title="Direct messages">
        <EmptyState title="No conversations" description="Direct conversations will appear here." />
      </Panel>
    </FeaturePage>
  );
}
