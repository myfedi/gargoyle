package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

type PushDeliveryWorker struct {
	jobs       repos.PushDeliveryJobsRepository
	subs       repos.PushSubscriptionRepository
	publicKey  string
	privateKey string
	subject    string
	logger     *log.Logger
	interval   time.Duration
	batchSize  int
	httpClient *http.Client
}

type PushDeliveryWorkerConfig struct {
	JobsRepo        repos.PushDeliveryJobsRepository
	Subscriptions   repos.PushSubscriptionRepository
	VAPIDPublicKey  string
	VAPIDPrivateKey string
	VAPIDSubject    string
	Logger          *log.Logger
	Interval        time.Duration
	BatchSize       int
}

func NewPushDeliveryWorker(cfg PushDeliveryWorkerConfig) *PushDeliveryWorker {
	if cfg.JobsRepo == nil {
		panic("push delivery worker requires JobsRepo")
	}
	if cfg.Subscriptions == nil {
		panic("push delivery worker requires Subscriptions")
	}
	if cfg.Logger == nil {
		cfg.Logger = log.Default()
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 5 * time.Second
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 25
	}
	return &PushDeliveryWorker{jobs: cfg.JobsRepo, subs: cfg.Subscriptions, publicKey: cfg.VAPIDPublicKey, privateKey: cfg.VAPIDPrivateKey, subject: cfg.VAPIDSubject, logger: cfg.Logger, interval: cfg.Interval, batchSize: cfg.BatchSize, httpClient: &http.Client{Timeout: 10 * time.Second}}
}

func (w *PushDeliveryWorker) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()
		for {
			w.ProcessOnce(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}
func (w *PushDeliveryWorker) ProcessOnce(ctx context.Context) {
	if w.publicKey == "" || w.privateKey == "" {
		return
	}
	jobs, err := w.jobs.ClaimDuePushDeliveryJobs(ctx, nil, time.Now().UTC(), w.batchSize)
	if err != nil {
		if ctx.Err() == nil {
			w.logger.Printf("push worker claim failed: %v", err)
		}
		return
	}
	for _, job := range jobs {
		w.processJob(ctx, job)
	}
}

func (w *PushDeliveryWorker) processJob(ctx context.Context, job models.PushDeliveryJob) {
	sub, notification, err := w.jobs.GetPushDeliveryPayload(ctx, nil, job)
	if err != nil {
		w.markFailed(ctx, job, err)
		return
	}
	payload, err := json.Marshal(map[string]any{"access_token": sub.AccessTokenID, "notification_id": notification.ID, "notification_type": notification.Type, "status_id": notification.StatusID, "title": pushTitle(notification.Type), "body": pushBody(notification.Type), "icon": "", "preferred_locale": "en"})
	if err != nil {
		w.markFailed(ctx, job, err)
		return
	}
	resp, err := webpush.SendNotificationWithContext(ctx, payload, &webpush.Subscription{Endpoint: sub.Endpoint, Keys: webpush.Keys{P256dh: sub.KeyP256DH, Auth: sub.KeyAuth}}, &webpush.Options{HTTPClient: w.httpClient, Subscriber: w.subject, TTL: 60, VAPIDPublicKey: w.publicKey, VAPIDPrivateKey: w.privateKey})
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		w.markFailed(ctx, job, err)
		return
	}
	if resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusNotFound {
		_ = w.subs.DeletePushSubscription(ctx, nil, sub.ID)
		w.markDelivered(ctx, job.ID)
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		w.markFailed(ctx, job, fmt.Errorf("push endpoint returned %s", resp.Status))
		return
	}
	w.markDelivered(ctx, job.ID)
}

func pushTitle(typ string) string {
	switch typ {
	case "mention":
		return "New mention"
	case "follow":
		return "New follower"
	case "follow_request":
		return "New follow request"
	case "favourite":
		return "New favourite"
	case "reblog":
		return "New boost"
	default:
		return "New notification"
	}
}
func pushBody(typ string) string {
	switch typ {
	case "mention":
		return "Someone mentioned you"
	case "follow":
		return "Someone followed you"
	case "follow_request":
		return "Someone requested to follow you"
	case "favourite":
		return "Someone favourited your status"
	case "reblog":
		return "Someone boosted your status"
	default:
		return "Open the app to view it"
	}
}
func (w *PushDeliveryWorker) markDelivered(ctx context.Context, id string) {
	if err := w.jobs.MarkPushDeliveryJobDelivered(ctx, nil, id, time.Now().UTC()); err != nil {
		w.logger.Printf("push job %s mark delivered failed: %v", id, err)
	}
}
func (w *PushDeliveryWorker) markFailed(ctx context.Context, job models.PushDeliveryJob, cause error) {
	attempts := job.Attempts + 1
	if err := w.jobs.MarkPushDeliveryJobFailed(ctx, nil, job.ID, attempts, nextAttempt(attempts), cause.Error()); err != nil {
		w.logger.Printf("push job %s mark failed error failed: %v", job.ID, err)
	}
}
