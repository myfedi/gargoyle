import type React from "react";
import { useEffect, useState, useSyncExternalStore } from "react";
import { Bell, LogOut, Menu, Search, Settings } from "lucide-react";

import { AuthProvider, useAuth } from "@/app/auth-context";
import { PoweredByFooter } from "@/components/powered-by-footer";
import { Button } from "@/components/ui/button";
import { DomainModerationPage } from "@/features/admin/domain-moderation-page";
import { LoginPage } from "@/features/auth/login-page";
import { AccountPage } from "@/features/accounts/account-page";
import { DirectMessagesPage } from "@/features/direct/direct-messages-page";
import { NotificationsPage } from "@/features/notifications/notifications-page";
import { PostsPage } from "@/features/posts/posts-page";
import { MyProfilePage } from "@/features/profile/my-profile-page";
import { SearchPopover } from "@/features/search/search-popover";
import { SettingsPage } from "@/features/settings/settings-page";
import { StatusPage } from "@/features/status/status-page";
import { ApiError } from "@/lib/api";
import { createMastodonApi } from "@/lib/mastodon-api";
import { startNotificationStream } from "@/lib/streaming";
import { cn } from "@/lib/utils";
import type { MastodonAccount } from "@/types/mastodon";
import { navItems } from "./navigation";

const routes = {
  "/profile": MyProfilePage,
  "/notifications": NotificationsPage,
  "/direct": DirectMessagesPage,
  "/settings": SettingsPage,
  "/admin/moderation/domains": DomainModerationPage,
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
  const [isSearchOpen, setIsSearchOpen] = useState(false);
  const [liveNotificationCount, setLiveNotificationCount] = useState(0);
  const route = useHashRoute();
  const RoutePage = routes[route as keyof typeof routes];
  const page = renderRoute(route, RoutePage);
  useEffect(() => {
    if (!("scrollRestoration" in globalThis.history)) {
      return;
    }
    const previous = globalThis.history.scrollRestoration;
    globalThis.history.scrollRestoration = "manual";
    return () => {
      globalThis.history.scrollRestoration = previous;
    };
  }, []);

  useEffect(() => {
    if (route === "/notifications") {
      setLiveNotificationCount(0);
    }
  }, [route]);

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

  useEffect(() => {
    if (status !== "authenticated" || !session?.accessToken) {
      setLiveNotificationCount(0);
      return;
    }
    return startNotificationStream(session.accessToken, {
      onNotification(notification) {
        globalThis.dispatchEvent(new CustomEvent("gargoyle:notification", { detail: notification }));
        setLiveNotificationCount((current) => current + 1);
      },
      onError(error) {
        console.warn("Notification stream disconnected", error);
      },
    });
  }, [session?.accessToken, status]);

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
    <div className="flex min-h-screen flex-col bg-background">
      <a
        href="#main-content"
        className="sr-only focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:z-50 focus:rounded-md focus:bg-primary focus:px-3 focus:py-2 focus:text-sm focus:text-primary-foreground"
      >
        Skip to content
      </a>

      <header className="sticky top-0 z-30 border-b border-border bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/80">
        <div className="mx-auto flex h-16 max-w-6xl items-center justify-between gap-3 px-4 md:px-6">
          <a href="/#/" className="min-w-0 rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2">
            <p className="text-sm font-semibold tracking-tight">Gargoyle</p>
            <p className="hidden truncate text-xs text-muted-foreground sm:block">
              {account ? `@${account.acct}` : accountError ? "Account unavailable" : "Personal federation"}
            </p>
          </a>

          <nav aria-label="Primary" className="hidden items-center gap-1 md:flex">
            {navItems.map((item) => {
              const Icon = item.icon;
              const isActive = item.href === `#${route}` || (item.href === "#/" && isTimelineRoute(route));
              return (
                <a
                  key={item.href}
                  href={item.href}
                  aria-current={isActive ? "page" : undefined}
                  className={cn(
                    "inline-flex items-center gap-2 rounded-md px-3 py-2 text-sm font-medium text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground",
                    isActive && "bg-secondary text-secondary-foreground",
                  )}
                >
                  <Icon className="size-4" aria-hidden="true" />
                  {item.label}
                </a>
              );
            })}
          </nav>

          <div className="flex items-center gap-2">
            <Button
              variant={isSearchOpen ? "secondary" : "ghost"}
              size="icon"
              aria-label="Search"
              aria-expanded={isSearchOpen}
              onClick={() => setIsSearchOpen((current) => !current)}
            >
              <Search className="size-4" aria-hidden="true" />
            </Button>
            <Button asChild variant="ghost" size="icon" aria-label={liveNotificationCount > 0 ? `${liveNotificationCount} new notifications` : "Notifications"}>
              <a href="/#/notifications" className="relative">
                <Bell className="size-4" aria-hidden="true" />
                {liveNotificationCount > 0 ? (
                  <span className="absolute -right-1 -top-1 min-w-4 rounded-full bg-primary px-1 text-[10px] font-semibold leading-4 text-primary-foreground">
                    {liveNotificationCount > 9 ? "9+" : liveNotificationCount}
                  </span>
                ) : null}
              </a>
            </Button>
            <Button asChild variant="ghost" size="icon" aria-label="Settings">
              <a href="/#/settings">
                <Settings className="size-4" aria-hidden="true" />
              </a>
            </Button>
            <Button variant="ghost" size="icon" aria-label="Sign out" onClick={signOut}>
              <LogOut className="size-4" aria-hidden="true" />
            </Button>
            <Button
              className="md:hidden"
              variant="outline"
              size="icon"
              aria-label="Open navigation"
              aria-expanded={isMobileNavOpen}
              onClick={() => setIsMobileNavOpen((current) => !current)}
            >
              <Menu className="size-4" aria-hidden="true" />
            </Button>
          </div>
        </div>
        {isSearchOpen ? <SearchPopover onClose={() => setIsSearchOpen(false)} /> : null}
        {isMobileNavOpen ? (
          <nav aria-label="Mobile primary" className="border-t border-border px-3 py-3 md:hidden">
            <div className="grid gap-1 sm:grid-cols-2">
              {navItems.map((item) => {
                const Icon = item.icon;
                const isActive = item.href === `#${route}` || (item.href === "#/" && isTimelineRoute(route));
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

      <main id="main-content" className="mx-auto w-full max-w-6xl flex-1 px-4 py-6 md:px-6 md:py-8">
        {page}
      </main>
      <PoweredByFooter />
    </div>
  );
}

function renderRoute(route: string, RoutePage: React.ComponentType | undefined) {
  if (isTimelineRoute(route)) {
    return <PostsPage route={route} />;
  }

  if (route.startsWith("/accounts/")) {
    return <AccountPage route={route} />;
  }

  if (route.startsWith("/statuses/")) {
    return <StatusPage route={route} />;
  }

  if (!RoutePage) {
    return (
      <section className="rounded-lg border border-border bg-card p-6 shadow-sm">
        <h1 className="text-xl font-semibold">Not found</h1>
        <p className="mt-2 text-sm text-muted-foreground">That page does not exist.</p>
        <Button asChild className="mt-5">
          <a href="/#/">Go to timeline</a>
        </Button>
      </section>
    );
  }

  const Page = RoutePage;
  return <Page />;
}

function useHashRoute() {
  return useSyncExternalStore(subscribeToHashChange, getHashRoute, getHashRoute);
}

function subscribeToHashChange(callback: () => void) {
  globalThis.addEventListener("hashchange", callback);
  return () => globalThis.removeEventListener("hashchange", callback);
}

function getHashRoute() {
  const route = globalThis.location.hash.replace(/^#/, "") || "/";
  if (route.startsWith("/")) {
    return route;
  }
  return isTimelineRoute(`/${route}`) ? `/${route}` : "/";
}

function isTimelineRoute(route: string) {
  const path = route.split("?")[0];
  return path === "/" || path === "/home" || path === "/local" || path === "/global";
}
