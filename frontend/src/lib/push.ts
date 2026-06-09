import type { MastodonPushSubscription } from "@/types/mastodon";

export type PushSupport = {
  supported: boolean;
  reason?: string;
};

export type PushAlertSettings = {
  mention: boolean;
  status: boolean;
  reblog: boolean;
  follow: boolean;
  follow_request: boolean;
  favourite: boolean;
  poll: boolean;
  update: boolean;
};

export const defaultPushAlerts: PushAlertSettings = {
  mention: true,
  status: false,
  reblog: true,
  follow: true,
  follow_request: true,
  favourite: true,
  poll: true,
  update: false,
};

export function pushSupport(): PushSupport {
  if (!("serviceWorker" in navigator)) {
    return { supported: false, reason: "This browser does not support service workers." };
  }
  if (!("PushManager" in window)) {
    return { supported: false, reason: "This browser does not support push notifications." };
  }
  if (!("Notification" in window)) {
    return { supported: false, reason: "This browser does not support notifications." };
  }
  if (!globalThis.isSecureContext) {
    return { supported: false, reason: "Push notifications require HTTPS." };
  }
  return { supported: true };
}

export async function ensurePushSubscription(serverKey: string) {
  const support = pushSupport();
  if (!support.supported) {
    throw new Error(support.reason ?? "Push notifications are not supported.");
  }
  if (!serverKey) {
    throw new Error("Push notifications are not configured on this server.");
  }

  const permission = await Notification.requestPermission();
  if (permission !== "granted") {
    throw new Error("Notifications were not allowed.");
  }

  const registration = await navigator.serviceWorker.register("/push-worker.js");
  await navigator.serviceWorker.ready;

  const existing = await registration.pushManager.getSubscription();
  if (existing) {
    return existing;
  }

  return registration.pushManager.subscribe({
    userVisibleOnly: true,
    applicationServerKey: urlBase64ToUint8Array(serverKey),
  });
}

export function subscriptionToServerPayload(subscription: PushSubscription, alerts: PushAlertSettings) {
  const json = subscription.toJSON();
  const endpoint = json.endpoint;
  const p256dh = json.keys?.p256dh;
  const auth = json.keys?.auth;
  if (!endpoint || !p256dh || !auth) {
    throw new Error("Browser returned an incomplete push subscription.");
  }

  return {
    subscription: { endpoint, keys: { p256dh, auth } },
    data: { alerts, policy: "all" },
  };
}

export async function unsubscribeBrowserPush() {
  if (!("serviceWorker" in navigator)) {
    return;
  }
  const registration = await navigator.serviceWorker.getRegistration("/");
  const subscription = await registration?.pushManager.getSubscription();
  await subscription?.unsubscribe();
}

export function alertsFromSubscription(subscription: MastodonPushSubscription | null): PushAlertSettings {
  return { ...defaultPushAlerts, ...(subscription?.alerts ?? {}) };
}

function urlBase64ToUint8Array(value: string) {
  const padding = "=".repeat((4 - (value.length % 4)) % 4);
  const base64 = (value + padding).replace(/-/g, "+").replace(/_/g, "/");
  const raw = globalThis.atob(base64);
  return Uint8Array.from([...raw].map((char) => char.charCodeAt(0)));
}
