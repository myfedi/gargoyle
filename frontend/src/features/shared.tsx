import type React from "react";

import { Inbox } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export type FeatureStatus = "ready" | "planned" | "needs-api";

type FeaturePageProps = {
  eyebrow: string;
  title: string;
  description: string;
  status?: FeatureStatus;
  primaryAction?: string;
  children?: React.ReactNode;
};

const statusLabels: Record<FeatureStatus, string> = {
  ready: "Ready for wiring",
  planned: "Planned",
  "needs-api": "Needs API",
};

export function FeaturePage({
  eyebrow,
  title,
  description,
  status = "planned",
  primaryAction,
  children,
}: FeaturePageProps) {
  return (
    <section className="space-y-8">
      <div className="flex flex-col gap-5 border-b border-border pb-7 md:flex-row md:items-end md:justify-between">
        <div className="max-w-3xl space-y-3">
          <div className="flex flex-wrap items-center gap-2">
            <p className="text-sm font-medium text-muted-foreground">{eyebrow}</p>
            <Badge variant={status === "ready" ? "success" : status === "needs-api" ? "warning" : "secondary"}>
              {statusLabels[status]}
            </Badge>
          </div>
          <div className="space-y-2">
            <h1 className="text-3xl font-semibold tracking-tight text-foreground">{title}</h1>
            <p className="max-w-2xl text-base leading-7 text-muted-foreground">{description}</p>
          </div>
        </div>
        {primaryAction ? <Button>{primaryAction}</Button> : null}
      </div>
      {children}
    </section>
  );
}

type PanelProps = {
  title: string;
  description?: string;
  children: React.ReactNode;
  className?: string;
};

export function Panel({ title, description, children, className }: PanelProps) {
  return (
    <div className={cn("rounded-lg border border-border bg-card p-5 shadow-sm", className)}>
      <div className="space-y-1">
        <h2 className="text-base font-semibold">{title}</h2>
        {description ? <p className="text-sm leading-6 text-muted-foreground">{description}</p> : null}
      </div>
      <div className="mt-5">{children}</div>
    </div>
  );
}

type EmptyStateProps = {
  title: string;
  description: string;
  action?: React.ReactNode;
};

export function EmptyState({ title, description, action }: EmptyStateProps) {
  return (
    <div className="flex min-h-44 flex-col items-center justify-center rounded-lg border border-dashed border-border bg-background px-6 py-8 text-center">
      <div className="rounded-full bg-secondary p-3 text-secondary-foreground">
        <Inbox className="size-5" aria-hidden="true" />
      </div>
      <h3 className="mt-4 text-sm font-semibold">{title}</h3>
      <p className="mt-2 max-w-md text-sm leading-6 text-muted-foreground">{description}</p>
      {action ? <div className="mt-5">{action}</div> : null}
    </div>
  );
}

type FieldRowProps = {
  label: string;
  value: React.ReactNode;
};

export function FieldRow({ label, value }: FieldRowProps) {
  return (
    <div className="grid gap-1 border-b border-border py-3 text-sm last:border-b-0 sm:grid-cols-[11rem_1fr] sm:gap-4">
      <dt className="font-medium text-muted-foreground">{label}</dt>
      <dd className="min-w-0 break-words text-foreground">{value}</dd>
    </div>
  );
}
