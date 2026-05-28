import { EmptyState, FeaturePage, Panel } from "@/features/shared";

export function InboxPage() {
  return (
    <FeaturePage
      eyebrow="Federation"
      title="Inbox"
      description="Inbound ActivityPub events should be summarized for humans, with raw payload inspection kept behind an explicit detail view."
      status="needs-api"
    >
      <Panel title="Inbound activity" description="This needs a Gargoyle-specific authenticated inbox activity endpoint.">
        <EmptyState
          title="No inbox API connected"
          description="When available, this screen should show Follow, Create, Delete, Update, Accept, and Reject activity summaries."
        />
      </Panel>
    </FeaturePage>
  );
}
