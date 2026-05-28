import type React from "react";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import type { MastodonStatus } from "@/types/mastodon";

const maxReplyLength = 500;

type ReplyComposerProps = {
  status: MastodonStatus;
  isSubmitting: boolean;
  error?: string | null;
  onCancel: () => void;
  onSubmit: (text: string) => Promise<void>;
};

export function ReplyComposer({ status, isSubmitting, error, onCancel, onSubmit }: ReplyComposerProps) {
  const [text, setText] = useState(prefillMention(status));
  const remaining = maxReplyLength - text.length;

  async function submitReply(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!text.trim() || remaining < 0) {
      return;
    }
    await onSubmit(text.trim());
  }

  return (
    <form className="space-y-3 rounded-lg border border-border bg-background p-4" onSubmit={(event) => void submitReply(event)}>
      <p className="text-sm font-medium">Reply to @{status.account.acct}</p>
      <Textarea value={text} onChange={(event) => setText(event.target.value)} rows={4} aria-label="Reply text" />
      {error ? (
        <p className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive" role="alert">
          {error}
        </p>
      ) : null}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <p className={remaining < 0 ? "text-sm text-destructive" : "text-sm text-muted-foreground"}>
          {remaining} characters remaining
        </p>
        <div className="flex gap-2">
          <Button type="button" variant="outline" onClick={onCancel} disabled={isSubmitting}>Cancel</Button>
          <Button type="submit" disabled={isSubmitting || !text.trim() || remaining < 0}>
            {isSubmitting ? "Replying..." : "Reply"}
          </Button>
        </div>
      </div>
    </form>
  );
}

function prefillMention(status: MastodonStatus) {
  return status.account.acct ? `@${status.account.acct} ` : "";
}
