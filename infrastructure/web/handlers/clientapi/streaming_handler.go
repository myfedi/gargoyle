package clientapi

import (
	"bufio"
	"encoding/json"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	clientapiUC "github.com/myfedi/gargoyle/domain/usecases/clientapi"
	"github.com/myfedi/gargoyle/infrastructure/web"
)

func (h APIHandler) streaming(c *fiber.Ctx) error {
	stream := c.Query("stream")
	if stream == "user:notification" || stream == "user" {
		return h.notificationStream(c)
	}
	return h.notificationStream(c)
}

func (h APIHandler) notificationStream(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c, "read")
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}

	initial, derr := h.notificationsWorkflow.Notifications(c.UserContext(), principal.Account, 40)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	initialResponses := notificationItemsToResponses(initial)
	client := h.realtimeHub.Add(principal)
	h.realtimeHub.MarkNotificationsSeen(client, initialResponses)
	for _, id := range strings.Split(c.Query("watch_relationships"), ",") {
		h.realtimeHub.WatchRelationship(client, strings.TrimSpace(id))
	}

	c.Set(fiber.HeaderContentType, "text/event-stream")
	c.Set(fiber.HeaderCacheControl, "no-cache, no-transform")
	c.Set(fiber.HeaderConnection, "keep-alive")
	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		defer func() {
			h.realtimeHub.Remove(client)
			_ = recover()
		}()

		_, _ = w.WriteString(": connected\n\n")
		_ = w.Flush()
		heartbeat := time.NewTicker(25 * time.Second)
		defer heartbeat.Stop()
		for {
			select {
			case <-heartbeat.C:
				_, _ = w.WriteString(": keep-alive\n\n")
				if err := w.Flush(); err != nil {
					return
				}
			case event := <-client.send:
				payload, err := json.Marshal(event.Data)
				if err != nil {
					continue
				}
				_, _ = w.WriteString("event: ")
				_, _ = w.WriteString(event.Event)
				_, _ = w.WriteString("\n")
				_, _ = w.WriteString("data: ")
				_, _ = w.Write(payload)
				_, _ = w.WriteString("\n\n")
				if err := w.Flush(); err != nil {
					return
				}
			}
		}
	})
	return nil
}

func notificationItemsToResponses(items []clientapiUC.NotificationItem) []notificationResponse {
	resp := make([]notificationResponse, 0, len(items))
	for _, item := range items {
		var status *statusResponse
		if item.Status != nil {
			s := timelineItemsToStatuses([]clientapiUC.TimelineItem{*item.Status})[0]
			status = &s
		}
		resp = append(resp, notificationResponse{ID: item.Notification.ID, Type: item.Notification.Type, CreatedAt: item.Notification.CreatedAt.UTC().Format(time.RFC3339), Account: accountToResponse(&item.Account), Status: status})
	}
	return resp
}
