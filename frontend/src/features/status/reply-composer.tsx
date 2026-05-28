import { ComposeForm, type ComposeValues } from "@/features/status/compose-form";
import type { MastodonStatus } from "@/types/mastodon";

type ReplyComposerProps = {
  status: MastodonStatus;
  isSubmitting: boolean;
  error?: string | null;
  onCancel: () => void;
  onSubmit: (values: ComposeValues) => Promise<void>;
};

export function ReplyComposer({ status, isSubmitting, error, onCancel, onSubmit }: ReplyComposerProps) {
  return (
    <div className="space-y-3 rounded-lg border border-border bg-background p-4">
      <div className="flex items-center justify-between gap-3">
        <p className="text-sm font-medium">Reply to @{status.account.acct}</p>
        <button type="button" className="text-sm text-muted-foreground hover:text-foreground" onClick={onCancel}>
          Cancel
        </button>
      </div>
      <ComposeForm
        submitLabel="Reply"
        submittingLabel="Replying..."
        placeholder="Write a reply"
        initialText={prefillMention(status)}
        isSubmitting={isSubmitting}
        error={error}
        onSubmit={onSubmit}
      />
    </div>
  );
}

function prefillMention(status: MastodonStatus) {
  return status.account.acct ? `@${status.account.acct} ` : "";
}
