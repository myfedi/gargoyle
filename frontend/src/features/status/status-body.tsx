import type { ReactNode } from "react";
import { useId, useState } from "react";

import { Button } from "@/components/ui/button";
import { StatusContent } from "@/features/status/status-content";
import type { MastodonMention } from "@/types/mastodon";

type StatusBodyProps = {
  html: string;
  mentions?: MastodonMention[];
  spoilerText?: string;
  children?: ReactNode;
};

export function StatusBody({ html, mentions, spoilerText = "", children }: StatusBodyProps) {
  const hasSpoiler = spoilerText.trim().length > 0;
  const [expanded, setExpanded] = useState(!hasSpoiler);
  const contentId = useId();

  if (!hasSpoiler) {
    return (
      <>
        <div className="mt-2">
          <StatusContent html={html} mentions={mentions} />
        </div>
        {children}
      </>
    );
  }

  return (
    <div className="mt-2 rounded-md border border-border bg-muted/30 p-3">
      <div className="flex flex-wrap items-center gap-3">
        <p className="min-w-0 flex-1 text-sm font-medium">{spoilerText}</p>
        <Button type="button" variant="outline" size="sm" aria-expanded={expanded} aria-controls={contentId} onClick={() => setExpanded((value) => !value)}>
          {expanded ? "Hide content" : "Show content"}
        </Button>
      </div>
      <div id={contentId} hidden={!expanded} className="mt-3">
        <StatusContent html={html} mentions={mentions} />
        {children}
      </div>
    </div>
  );
}
