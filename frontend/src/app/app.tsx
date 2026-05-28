import type React from "react";
import { useEffect, useMemo, useState, useSyncExternalStore } from "react";
import { LogOut, Menu } from "lucide-react";

import { AuthProvider, useAuth } from "@/app/auth-context";
import { Button } from "@/components/ui/button";
import { LoginPage } from "@/features/auth/login-page";
import { CompatibilityPage } from "@/features/compatibility/compatibility-page";
import { DeliveryPage } from "@/features/delivery/delivery-page";
import { FollowsPage } from "@/features/follows/follows-page";
import { InboxPage } from "@/features/inbox/inbox-page";
import { OutboxPage } from "@/features/outbox/outbox-page";
import { OverviewPage } from "@/features/overview/overview-page";
import { PostsPage } from "@/features/posts/posts-page";
import { SettingsPage } from "@/features/settings/settings-page";
import { ApiError } from "@/lib/api";
import { createMastodonApi } from "@/lib/mastodon-api";
import { cn } from "@/lib/utils";
import type { MastodonAccount } from "@/types/mastodon";
import { navItems } from "./navigation";

const routes = {
  "/": OverviewPage,
  "/posts": PostsPage,
  "/follows": FollowsPage,
  "/inbox": InboxPage,
  "/outbox": OutboxPage,
  "/delivery": DeliveryPage,
  "/compatibility": CompatibilityPage,
  "/settings": SettingsPage,
} satisfies Record<string, React.ComponentType>;

export function App() {
  return (
    <AuthProvider>
      <AuthenticatedApp />
    </AuthProvider>
  );
}

function AuthenticatedApp() {
  const { session, status, signOut } = useAuth();
  const [account, setAccount] = useState<MastodonAccount | null>(null);
  const [accountError, setAccountError] = useState<string | null>(null);
  const [isMobileNavOpen, setIsMobileNavOpen] = useState(false);
  const route = useHashRoute();
  const Page = routes[route as keyof typeof routes] ?? OverviewPage;
  const currentItem = useMemo(
    () => navItems.find((item) => item.href === `#${route}`) ?? navItems[0],
    [route],
  );

  useEffect(() => {
    if (status !== "authenticated" || !session?.accessToken) {
      setAccount(null);
      return;
    }

    let cancelled = false;
    const mastodonApi = createMastodonApi(session.accessToken);

    mastodonApi
      .verifyCredentials()
      .then((nextAccount) => {
        if (!cancelled) {
          setAccount(nextAccount);
          setAccountError(null);
        }
      })
      .catch((caughtError: unknown) => {
        if (cancelled) {
          return;
        }

        if (caughtError instanceof ApiError && caughtError.status === 401) {
          signOut();
          return;
        }

        setAccountError(caughtError instanceof Error ? caughtError.message : "Could not load signed-in account.");
      });

    return () => {
      cancelled = true;
    };
  }, [session?.accessToken, signOut, status]);

  if (status === "checking") {
    return (
      <main className="flex min-h-screen items-center justify-center bg-background px-4">
        <p className="text-sm text-muted-foreground">Checking authorization...</p>
      </main>
    );
  }

  if (status === "unauthenticated") {
    return <LoginPage />;
  }

  return (
    <div className="min-h-screen bg-background">
      <a
        href="#main-content"
        className="sr-only focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:z-50 focus:rounded-md focus:bg-primary focus:px-3 focus:py-2 focus:text-sm focus:text-primary-foreground"
      >
        Skip to content
      </a>

      <div className="grid min-h-screen lg:grid-cols-[17rem_1fr]">
        <aside className="hidden border-r border-border bg-card/70 lg:block">
          <div className="sticky top-0 flex h-screen flex-col px-4 py-5">
            <div className="px-2 pb-6">
              <p className="text-lg font-semibold tracking-tight">Gargoyle</p>
              <p className="mt-1 text-sm text-muted-foreground">Personal federation console</p>
              <div className="mt-4 rounded-lg border border-border bg-background px-3 py-2">
                <p className="truncate text-sm font-medium">{account?.display_name || account?.username || "Signed in"}</p>
                <p className="truncate text-xs text-muted-foreground">
                  {account ? `@${account.acct}` : accountError ? "Account check failed" : "Checking account..."}
                </p>
              </div>
            </div>
            <nav aria-label="Primary" className="space-y-1">
              {navItems.map((item) => {
                const Icon = item.icon;
                const isActive = item.href === `#${route}`;
                return (
                  <a
                    key={item.href}
                    href={item.href}
                    aria-current={isActive ? "page" : undefined}
                    className={cn(
                      "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground",
                      isActive && "bg-secondary text-secondary-foreground",
                    )}
                  >
                    <Icon className="size-4" aria-hidden="true" />
                    {item.label}
                  </a>
                );
              })}
            </nav>
            <div className="mt-auto space-y-3">
              <Button variant="outline" className="w-full justify-start" onClick={signOut}>
                <LogOut className="size-4" aria-hidden="true" />
                Sign out
              </Button>
            </div>
          </div>
        </aside>

        <div className="flex min-w-0 flex-col">
          <header className="sticky top-0 z-20 border-b border-border bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/80 lg:hidden">
            <div className="flex h-14 items-center justify-between px-4">
              <div>
                <p className="text-sm font-semibold">Gargoyle</p>
                <p className="text-xs text-muted-foreground">{currentItem?.label}</p>
              </div>
              <div className="flex items-center gap-2">
                <Button
                  variant="outline"
                  size="icon"
                  aria-label="Open navigation"
                  aria-expanded={isMobileNavOpen}
                  onClick={() => setIsMobileNavOpen((current) => !current)}
                >
                  <Menu className="size-4" aria-hidden="true" />
                </Button>
                <Button variant="outline" size="icon" aria-label="Sign out" onClick={signOut}>
                  <LogOut className="size-4" aria-hidden="true" />
                </Button>
              </div>
            </div>
            {isMobileNavOpen ? (
              <nav aria-label="Mobile primary" className="border-t border-border px-3 py-3">
                <div className="grid gap-1 sm:grid-cols-2">
                  {navItems.map((item) => {
                    const Icon = item.icon;
                    const isActive = item.href === `#${route}`;
                    return (
                      <a
                        key={item.href}
                        href={item.href}
                        aria-current={isActive ? "page" : undefined}
                        onClick={() => setIsMobileNavOpen(false)}
                        className={cn(
                          "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground",
                          isActive && "bg-secondary text-secondary-foreground",
                        )}
                      >
                        <Icon className="size-4" aria-hidden="true" />
                        {item.label}
                      </a>
                    );
                  })}
                </div>
              </nav>
            ) : null}
          </header>

          <main id="main-content" className="w-full px-4 py-6 md:px-8 md:py-8 xl:px-10">
            <Page />
          </main>
        </div>
      </div>
    </div>
  );
}

function useHashRoute() {
  return useSyncExternalStore(subscribeToHashChange, getHashRoute, getHashRoute);
}

function subscribeToHashChange(callback: () => void) {
  window.addEventListener("hashchange", callback);
  return () => window.removeEventListener("hashchange", callback);
}

function getHashRoute() {
  const route = window.location.hash.replace(/^#/, "") || "/";
  return route.startsWith("/") ? route : "/";
}
