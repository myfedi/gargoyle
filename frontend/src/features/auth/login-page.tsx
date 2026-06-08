import { LockKeyhole, ShieldCheck } from "lucide-react";

import { PoweredByFooter } from "@/components/powered-by-footer";
import { Button } from "@/components/ui/button";
import { useAuth } from "@/app/auth-context";
import { getOAuthConfig } from "@/lib/config";

export function LoginPage() {
  const { error, signIn, status } = useAuth();
  const oauthConfig = getOAuthConfig();

  return (
    <div className="flex min-h-screen flex-col bg-background">
      <main className="flex flex-1 items-center justify-center px-4 py-10">
        <section className="w-full max-w-md rounded-xl border border-border bg-card p-6 shadow-sm" aria-labelledby="login-title">
        <div className="mb-7 flex items-start gap-4">
          <div className="rounded-lg bg-secondary p-3 text-secondary-foreground">
            <LockKeyhole className="size-5" aria-hidden="true" />
          </div>
          <div>
            <p className="text-sm font-medium text-muted-foreground">Gargoyle</p>
            <h1 id="login-title" className="mt-1 text-2xl font-semibold tracking-tight">
              Authorize this console
            </h1>
          </div>
        </div>

        <p className="text-sm leading-6 text-muted-foreground">
          Sign in to manage posts, follows, delivery, and federation activity for this Gargoyle instance.
        </p>

        <div className="mt-6 rounded-lg border border-border bg-background p-4">
          <div className="flex gap-3">
            <ShieldCheck className="mt-0.5 size-4 shrink-0 text-primary" aria-hidden="true" />
            <div className="space-y-1">
              <p className="text-sm font-medium">Secure instance access</p>
              <p className="text-sm leading-6 text-muted-foreground">
                You will be sent to Gargoyle to approve this console, then brought back here.
              </p>
            </div>
          </div>
        </div>

        {error ? (
          <p className="mt-4 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive" role="alert">
            {error}
          </p>
        ) : null}

        {!oauthConfig ? (
          <p className="mt-4 rounded-md border border-amber-300/70 bg-amber-100/70 px-3 py-2 text-sm text-amber-950" role="alert">
            This console is not configured for sign-in yet.
          </p>
        ) : null}

        <Button className="mt-6 w-full" onClick={() => void signIn()} disabled={!oauthConfig || status === "checking"}>
          Continue with Gargoyle
        </Button>
        </section>
      </main>
      <PoweredByFooter />
    </div>
  );
}
