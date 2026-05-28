package jobs

import (
	"context"
	"time"

	"github.com/myfedi/gargoyle/domain/ports/repos"
)

// FetchWorker is the durable fetch queue runner. Fetch execution is intentionally
// kept minimal until concrete fetch job kinds are introduced; this worker marks
// unknown jobs retryable instead of silently dropping them.
type FetchWorker struct {
	jobs      repos.FetchJobsRepository
	interval  time.Duration
	batchSize int
}

type FetchWorkerConfig struct {
	JobsRepo  repos.FetchJobsRepository
	Interval  time.Duration
	BatchSize int
}

func NewFetchWorker(cfg FetchWorkerConfig) *FetchWorker {
	if cfg.JobsRepo == nil {
		panic("fetch worker requires JobsRepo")
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 30 * time.Second
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 25
	}
	return &FetchWorker{jobs: cfg.JobsRepo, interval: cfg.Interval, batchSize: cfg.BatchSize}
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
		_ = w.jobs.MarkFetchJobFailed(ctx, nil, job.ID, job.Attempts+1, nextAttempt(job.Attempts+1), "no fetch handler registered for kind "+job.Kind)
	}
}
