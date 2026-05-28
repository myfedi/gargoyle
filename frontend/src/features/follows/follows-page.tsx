import { EmptyState, FeaturePage, Panel } from "@/features/shared";

export function FollowsPage() {
  return (
    <FeaturePage eyebrow="People" title="Follows" description="Followers and following.">
      <Panel title="Not implemented">
        <EmptyState
          title="Follow management is not implemented"
          description="Needed: followers list, following list, account search, follow, and unfollow."
        />
      </Panel>
    </FeaturePage>
  );
}
