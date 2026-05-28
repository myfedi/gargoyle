package jobs

import (
	"context"
	"time"

	"github.com/myfedi/gargoyle/domain/ports"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

// FetchWorker is the durable fetch queue runner. Fetch execution is intentionally
// kept minimal until concrete fetch job kinds are introduced; this worker marks
// unknown jobs retryable instead of silently dropping them.
type FetchWorker struct {
	jobs      repos.FetchJobsRepository
	accounts  repos.AccountsRepo
	fetcher   ports.RemoteObjectFetcher
	interval  time.Duration
	batchSize int
}

type FetchWorkerConfig struct {
	JobsRepo  repos.FetchJobsRepository
	Accounts  repos.AccountsRepo
	Fetcher   ports.RemoteObjectFetcher
	Interval  time.Duration
	BatchSize int
}

func NewFetchWorker(cfg FetchWorkerConfig) *FetchWorker {
	if cfg.JobsRepo == nil {
		panic("fetch worker requires JobsRepo")
	}
	if cfg.Accounts == nil {
		panic("fetch worker requires Accounts")
	}
	if cfg.Fetcher == nil {
		panic("fetch worker requires Fetcher")
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 30 * time.Second
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 25
	}
	return &FetchWorker{jobs: cfg.JobsRepo, accounts: cfg.Accounts, fetcher: cfg.Fetcher, interval: cfg.Interval, batchSize: cfg.BatchSize}
}

func (w *FetchWorker) Start(ctx context.Context) {
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

func (w *FetchWorker) ProcessOnce(ctx context.Context) {
	due, err := w.jobs.ListDueFetchJobs(ctx, nil, time.Now().UTC(), w.batchSize)
	if err != nil {
		return
	}
	for _, job := range due {
		account, err := w.accounts.GetAccountByID(ctx, nil, job.AccountID)
		if err != nil {
			_ = w.jobs.MarkFetchJobFailed(ctx, nil, job.ID, job.Attempts+1, nextAttempt(job.Attempts+1), err.Error())
			continue
		}
		if err := w.fetcher.FetchObject(ctx, job.URL, account); err != nil {
			_ = w.jobs.MarkFetchJobFailed(ctx, nil, job.ID, job.Attempts+1, nextAttempt(job.Attempts+1), err.Error())
			continue
		}
		_ = w.jobs.MarkFetchJobFetched(ctx, nil, job.ID, time.Now().UTC())
	}
}
