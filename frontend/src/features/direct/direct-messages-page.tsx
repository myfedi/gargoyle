import { DirectMessageForm } from "@/features/direct/direct-message-form";
import { EmptyState, FeaturePage, Panel } from "@/features/shared";

export function DirectMessagesPage() {
  return (
    <FeaturePage eyebrow="Messages" title="Direct messages" description="Private conversations.">
      <Panel title="New direct message">
        <DirectMessageForm />
      </Panel>
      <Panel title="Conversations">
        <EmptyState title="No conversations" description="Direct conversations will appear here." />
      </Panel>
    </FeaturePage>
  );
}
