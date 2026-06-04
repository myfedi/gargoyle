package jobs

import (
	"context"
	"log"
	"time"

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
	logger    *log.Logger
	interval  time.Duration
	batchSize int
}

type DeliveryWorkerConfig struct {
	JobsRepo  repos.DeliveryJobsRepository
	Accounts  repos.AccountsRepo
	Deliverer apPorts.ActivityDeliverer
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
	return &DeliveryWorker{jobs: cfg.JobsRepo, accounts: cfg.Accounts, deliverer: cfg.Deliverer, logger: cfg.Logger, interval: cfg.Interval, batchSize: cfg.BatchSize}
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

func (w *DeliveryWorker) ProcessOnce(ctx context.Context) {
	due, err := w.jobs.ClaimDueDeliveryJobs(ctx, nil, time.Now().UTC(), w.batchSize)
	if err != nil {
		w.logger.Printf("delivery worker claim failed: %v", err)
		return
	}
	for _, job := range due {
		account, err := w.accounts.GetAccountByID(ctx, nil, job.AccountID)
		if err != nil {
			w.logger.Printf("delivery job %s account lookup failed account_id=%s: %v", job.ID, job.AccountID, err)
			if markErr := w.jobs.MarkDeliveryJobFailed(ctx, nil, job.ID, job.Attempts+1, nextAttempt(job.Attempts+1), err.Error()); markErr != nil {
				w.logger.Printf("delivery job %s mark failed error failed: %v", job.ID, markErr)
			}
			continue
		}
		if err := w.deliverer.Deliver(ctx, job.Payload, job.InboxURL, *account); err != nil {
			w.logger.Printf("delivery job %s delivery failed inbox=%s attempts=%d: %v", job.ID, job.InboxURL, job.Attempts+1, err)
			if markErr := w.jobs.MarkDeliveryJobFailed(ctx, nil, job.ID, job.Attempts+1, nextAttempt(job.Attempts+1), err.Error()); markErr != nil {
				w.logger.Printf("delivery job %s mark failed error failed: %v", job.ID, markErr)
			}
			continue
		}
		if err := w.jobs.MarkDeliveryJobDelivered(ctx, nil, job.ID, time.Now().UTC()); err != nil {
			w.logger.Printf("delivery job %s mark delivered failed: %v", job.ID, err)
		}
	}
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
