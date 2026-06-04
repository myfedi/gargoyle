package jobs

import (
	"context"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	apPorts "github.com/myfedi/gargoyle/domain/ports/activitypub"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

// DeliveryWorker processes persisted ActivityPub delivery jobs. It is an
// infrastructure adapter: jobs are stored through repository ports and actual
// network delivery is delegated to the ActivityPub delivery port.
type DeliveryWorker struct {
	jobs      repos.DeliveryJobsRepository
	accounts  repos.AccountsRepo
	deliverer apPorts.ActivityDeliverer
	blocks    repos.DomainBlocksRepository
	logger    *log.Logger
	interval  time.Duration
	batchSize int
}

type DeliveryWorkerConfig struct {
	JobsRepo  repos.DeliveryJobsRepository
	Accounts  repos.AccountsRepo
	Deliverer apPorts.ActivityDeliverer
	Blocks    repos.DomainBlocksRepository
	Logger    *log.Logger
	Interval  time.Duration
	BatchSize int
}

func NewDeliveryWorker(cfg DeliveryWorkerConfig) *DeliveryWorker {
	if cfg.JobsRepo == nil {
		panic("delivery worker requires JobsRepo")
	}
	if cfg.Accounts == nil {
		panic("delivery worker requires Accounts")
	}
	if cfg.Deliverer == nil {
		panic("delivery worker requires Deliverer")
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
	return &DeliveryWorker{jobs: cfg.JobsRepo, accounts: cfg.Accounts, deliverer: cfg.Deliverer, blocks: cfg.Blocks, logger: cfg.Logger, interval: cfg.Interval, batchSize: cfg.BatchSize}
}

func (w *DeliveryWorker) Start(ctx context.Context) {
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

// ProcessOnce claims durable delivery work and delegates each job to a small
// handler. The worker is infrastructure: ActivityPub delivery is still behind a
// port and job state is updated through repository ports.
func (w *DeliveryWorker) ProcessOnce(ctx context.Context) {
	due, err := w.jobs.ClaimDueDeliveryJobs(ctx, nil, time.Now().UTC(), w.batchSize)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		w.logger.Printf("delivery worker claim failed: %v", err)
		return
	}
	for _, job := range due {
		w.processJob(ctx, job)
	}
}

// processJob keeps the happy path linear: skip blocked inboxes, load signer,
// deliver, then mark final state. Each branch records a durable outcome.
func (w *DeliveryWorker) processJob(ctx context.Context, job models.DeliveryJob) {
	if w.deliveryBlocked(ctx, job.InboxURL) {
		w.logger.Printf("delivery job %s skipped blocked inbox=%s", job.ID, job.InboxURL)
		w.markDelivered(ctx, job.ID, "skipped delivered")
		return
	}

	account, err := w.accounts.GetAccountByID(ctx, nil, job.AccountID)
	if err != nil {
		w.logger.Printf("delivery job %s account lookup failed account_id=%s: %v", job.ID, job.AccountID, err)
		w.markFailed(ctx, job, err)
		return
	}

	if err := w.deliverer.Deliver(ctx, job.Payload, job.InboxURL, *account); err != nil {
		w.logger.Printf("delivery job %s delivery failed inbox=%s attempts=%d: %v", job.ID, job.InboxURL, job.Attempts+1, err)
		w.markFailed(ctx, job, err)
		return
	}
	w.markDelivered(ctx, job.ID, "delivered")
}

// markFailed applies retry bookkeeping in one place so claim/deliver logic does
// not need to know backoff details.
func (w *DeliveryWorker) markFailed(ctx context.Context, job models.DeliveryJob, cause error) {
	attempts := job.Attempts + 1
	if err := w.jobs.MarkDeliveryJobFailed(ctx, nil, job.ID, attempts, nextAttempt(attempts), cause.Error()); err != nil {
		w.logger.Printf("delivery job %s mark failed error failed: %v", job.ID, err)
	}
}

func (w *DeliveryWorker) markDelivered(ctx context.Context, jobID, action string) {
	if err := w.jobs.MarkDeliveryJobDelivered(ctx, nil, jobID, time.Now().UTC()); err != nil {
		w.logger.Printf("delivery job %s mark %s failed: %v", jobID, action, err)
	}
}

// deliveryBlocked enforces moderation blocks at the network boundary. Domain
// decisions come from the block repository; the worker only interprets URLs.
func (w *DeliveryWorker) deliveryBlocked(ctx context.Context, inbox string) bool {
	if w.blocks == nil {
		return false
	}
	parsed, err := url.Parse(inbox)
	if err != nil || parsed.Host == "" {
		return false
	}

	blocked, err := w.blocks.DomainIsSuspended(ctx, nil, strings.ToLower(parsed.Hostname()))
	return err == nil && blocked
}

func nextAttempt(attempts int) time.Time {
	if attempts < 1 {
		attempts = 1
	}
	backoff := time.Duration(attempts*attempts) * time.Minute
	if backoff > time.Hour {
		backoff = time.Hour
	}
	return time.Now().UTC().Add(backoff)
}
