self.addEventListener("push", (event) => {
  let payload = {};
  try {
    payload = event.data ? event.data.json() : {};
  } catch (error) {
    payload = {};
  }

  const title = payload.title || "New notification";
  const notificationId = payload.notification_id || "";
  const notificationType = payload.notification_type || "";
  const options = {
    body: payload.body || "Open Gargoyle to view it.",
    tag: notificationId || notificationType || "gargoyle-notification",
    renotify: Boolean(notificationId),
    icon: payload.icon || "/favicon.svg",
    badge: "/favicon.svg",
    data: {
      url: notificationType === "mention" && payload.status_id ? `/#/statuses/${payload.status_id}` : "/#/notifications",
    },
  };

  event.waitUntil(self.registration.showNotification(title, options));
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  const url = event.notification.data?.url || "/#/notifications";
  event.waitUntil((async () => {
    const allClients = await clients.matchAll({ type: "window", includeUncontrolled: true });
    for (const client of allClients) {
      if ("focus" in client) {
        await client.focus();
        if ("navigate" in client) {
          await client.navigate(url);
        }
        return;
      }
    }
    await clients.openWindow(url);
  })());
});
