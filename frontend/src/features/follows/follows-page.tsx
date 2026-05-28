import { EmptyState, FeaturePage, Panel } from "@/features/shared";

export function FollowsPage() {
  return (
    <FeaturePage
      eyebrow="Social graph"
      title="Follows"
      description="Followers and following belong here once Gargoyle exposes the corresponding Mastodon-compatible account relationship endpoints."
      status="needs-api"
    >
      <Panel title="People" description="No frontend guesses here. This screen is intentionally empty until real follow endpoints exist.">
        <EmptyState
          title="Follow APIs are not wired yet"
          description="Expected next endpoints are followers, following, account search or lookup, follow, and unfollow."
        />
      </Panel>
    </FeaturePage>
  );
}
