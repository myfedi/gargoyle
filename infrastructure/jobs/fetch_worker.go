package jobs

import (
	"context"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/myfedi/gargoyle/domain/ports/repos"
	apUsecases "github.com/myfedi/gargoyle/domain/usecases/activitypub"
)

// FetchWorker processes durable remote fetch jobs. It delegates object parsing
// and persistence to a domain use case so the worker stays an infrastructure
// scheduler rather than owning ActivityPub hydration rules.
type FetchWorker struct {
	jobs      repos.FetchJobsRepository
	accounts  repos.AccountsRepo
	hydrater  apUsecases.HydrateRemoteObjectUseCase
	blocks    repos.DomainBlocksRepository
	logger    *log.Logger
	interval  time.Duration
	batchSize int
}

type FetchWorkerConfig struct {
	JobsRepo  repos.FetchJobsRepository
	Accounts  repos.AccountsRepo
	Hydrater  apUsecases.HydrateRemoteObjectUseCase
	Blocks    repos.DomainBlocksRepository
	Logger    *log.Logger
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
	if cfg.Logger == nil {
		cfg.Logger = log.Default()
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 30 * time.Second
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 25
	}
	return &FetchWorker{jobs: cfg.JobsRepo, accounts: cfg.Accounts, hydrater: cfg.Hydrater, blocks: cfg.Blocks, logger: cfg.Logger, interval: cfg.Interval, batchSize: cfg.BatchSize}
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
	due, err := w.jobs.ClaimDueFetchJobs(ctx, nil, time.Now().UTC(), w.batchSize)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		w.logger.Printf("fetch worker claim failed: %v", err)
		return
	}
	for _, job := range due {
		if w.fetchBlocked(ctx, job.URL) {
			w.logger.Printf("fetch job %s skipped blocked url=%s", job.ID, job.URL)
			if err := w.jobs.MarkFetchJobFetched(ctx, nil, job.ID, time.Now().UTC()); err != nil {
				w.logger.Printf("fetch job %s mark skipped fetched failed: %v", job.ID, err)
			}
			continue
		}
		account, err := w.accounts.GetAccountByID(ctx, nil, job.AccountID)
		if err != nil {
			w.logger.Printf("fetch job %s account lookup failed account_id=%s: %v", job.ID, job.AccountID, err)
			if markErr := w.jobs.MarkFetchJobFailed(ctx, nil, job.ID, job.Attempts+1, nextAttempt(job.Attempts+1), err.Error()); markErr != nil {
				w.logger.Printf("fetch job %s mark failed error failed: %v", job.ID, markErr)
			}
			continue
		}
		if err := w.hydrater.HydrateRemoteObject(ctx, *account, job.URL); err != nil {
			w.logger.Printf("fetch job %s hydrate failed url=%s attempts=%d: %v", job.ID, job.URL, job.Attempts+1, err)
			if markErr := w.jobs.MarkFetchJobFailed(ctx, nil, job.ID, job.Attempts+1, nextAttempt(job.Attempts+1), err.Error()); markErr != nil {
				w.logger.Printf("fetch job %s mark failed error failed: %v", job.ID, markErr)
			}
			continue
		}
		if err := w.jobs.MarkFetchJobFetched(ctx, nil, job.ID, time.Now().UTC()); err != nil {
			w.logger.Printf("fetch job %s mark fetched failed: %v", job.ID, err)
		}
	}
}

func (w *FetchWorker) fetchBlocked(ctx context.Context, raw string) bool {
	if w.blocks == nil {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return false
	}
	blocked, err := w.blocks.DomainIsSuspended(ctx, nil, strings.ToLower(parsed.Hostname()))
	return err == nil && blocked
}
