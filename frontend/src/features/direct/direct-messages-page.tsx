import { FeaturePage, Panel, EmptyState } from "@/features/shared";

export function DirectMessagesPage() {
  return (
    <FeaturePage eyebrow="Messages" title="Direct messages" description="Private conversations.">
      <Panel title="Direct messages">
        <EmptyState
          title="Direct messages are not implemented"
          description="Mastodon direct statuses need a conversation view before this can be useful."
        />
      </Panel>
    </FeaturePage>
  );
}
