package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/ports"
	"github.com/myfedi/gargoyle/domain/ports/repos"
	clientapiUC "github.com/myfedi/gargoyle/domain/usecases/clientapi"
)

type ModerationWorkerConfig struct {
	JobsRepo     repos.ModerationJobsRepository
	API          clientapiUC.Moderation
	MediaStorage ports.MediaStorage
	Logger       *log.Logger
	Interval     time.Duration
	BatchSize    int
}

type ModerationWorker struct{ cfg ModerationWorkerConfig }

func NewModerationWorker(cfg ModerationWorkerConfig) *ModerationWorker {
	if cfg.JobsRepo == nil {
		panic("moderation worker requires JobsRepo")
	}
	if cfg.MediaStorage == nil {
		panic("moderation worker requires MediaStorage")
	}
	if cfg.Logger == nil {
		cfg.Logger = log.Default()
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 10 * time.Second
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 5
	}
	return &ModerationWorker{cfg: cfg}
}

func (w *ModerationWorker) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(w.cfg.Interval)
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

func (w *ModerationWorker) ProcessOnce(ctx context.Context) {
	due, err := w.cfg.JobsRepo.ClaimDueModerationJobs(ctx, nil, time.Now().UTC(), w.cfg.BatchSize)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		w.cfg.Logger.Printf("moderation worker claim failed: %v", err)
		return
	}
	for _, job := range due {
		if err := w.processJob(ctx, job); err != nil {
			w.cfg.Logger.Printf("moderation job %s failed kind=%s attempts=%d: %v", job.ID, job.Kind, job.Attempts+1, err)
			if markErr := w.cfg.JobsRepo.MarkModerationJobFailed(ctx, nil, job.ID, job.Attempts+1, nextAttempt(job.Attempts+1), err.Error()); markErr != nil {
				w.cfg.Logger.Printf("moderation job %s mark failed error failed: %v", job.ID, markErr)
			}
			continue
		}
		if err := w.cfg.JobsRepo.MarkModerationJobDone(ctx, nil, job.ID, time.Now().UTC()); err != nil {
			w.cfg.Logger.Printf("moderation job %s mark done failed: %v", job.ID, err)
		}
	}
}

func (w *ModerationWorker) processJob(ctx context.Context, job models.ModerationJob) error {
	switch job.Kind {
	case models.ModerationJobKindPurgeDomain:
		var payload clientapiUC.PurgeDomainPayload
		if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
			return err
		}
		result, storagePaths, err := w.cfg.API.PurgeDomain(ctx, payload.Domain)
		if err != nil {
			return err
		}
		for _, path := range storagePaths {
			if err := w.cfg.MediaStorage.DeleteMedia(ctx, path); err != nil {
				w.cfg.Logger.Printf("moderation job %s media cleanup failed path=%s: %v", job.ID, path, err)
			}
		}
		w.cfg.Logger.Printf("moderation job %s purged domain=%s notes=%d accounts=%d follows=%d notifications=%d media=%d", job.ID, result.Domain, result.DeletedNotes, result.DeletedRemoteAccounts, result.DeletedFollows, result.DeletedNotifications, result.DeletedMedia)
		return nil
	default:
		return fmt.Errorf("unsupported moderation job kind %q", job.Kind)
	}
}
