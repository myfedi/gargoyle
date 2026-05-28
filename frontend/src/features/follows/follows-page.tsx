import { EmptyState, FeaturePage, Panel } from "@/features/shared";

export function FollowsPage() {
  return (
    <FeaturePage
      eyebrow="People"
      title="Follows"
      description="See who is connected to this instance and manage relationships from one quiet, readable place."
      status="needs-api"
    >
      <Panel title="People" description="Follower and following management will appear here soon.">
        <EmptyState
          title="No follow list yet"
          description="Once this area is ready, you will be able to review followers, see who you follow, and manage relationships."
        />
      </Panel>
    </FeaturePage>
  );
}
