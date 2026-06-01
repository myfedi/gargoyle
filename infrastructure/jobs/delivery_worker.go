package jobs

import (
	"context"
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
	interval  time.Duration
	batchSize int
}

type DeliveryWorkerConfig struct {
	JobsRepo  repos.DeliveryJobsRepository
	Accounts  repos.AccountsRepo
	Deliverer apPorts.ActivityDeliverer
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
	if cfg.Interval <= 0 {
		cfg.Interval = 5 * time.Second
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 25
	}
	return &DeliveryWorker{jobs: cfg.JobsRepo, accounts: cfg.Accounts, deliverer: cfg.Deliverer, interval: cfg.Interval, batchSize: cfg.BatchSize}
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
		return
	}
	for _, job := range due {
		account, err := w.accounts.GetAccountByID(ctx, nil, job.AccountID)
		if err != nil {
			_ = w.jobs.MarkDeliveryJobFailed(ctx, nil, job.ID, job.Attempts+1, nextAttempt(job.Attempts+1), err.Error())
			continue
		}
		if err := w.deliverer.Deliver(ctx, job.Payload, job.InboxURL, *account); err != nil {
			_ = w.jobs.MarkDeliveryJobFailed(ctx, nil, job.ID, job.Attempts+1, nextAttempt(job.Attempts+1), err.Error())
			continue
		}
		_ = w.jobs.MarkDeliveryJobDelivered(ctx, nil, job.ID, time.Now().UTC())
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
