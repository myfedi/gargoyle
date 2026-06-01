package jobs

import (
	"context"
	"log"
	"time"

	"github.com/myfedi/gargoyle/domain/ports"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

type MediaCleanupWorkerConfig struct {
	MediaRepo     repos.MediaRepository
	Storage       ports.MediaStorage
	Interval      time.Duration
	UnattachedTTL time.Duration
	BatchSize     int
}

type MediaCleanupWorker struct{ cfg MediaCleanupWorkerConfig }

func NewMediaCleanupWorker(cfg MediaCleanupWorkerConfig) *MediaCleanupWorker {
	if cfg.MediaRepo == nil {
		panic("media cleanup worker requires MediaRepo")
	}
	if cfg.Storage == nil {
		panic("media cleanup worker requires Storage")
	}
	if cfg.Interval <= 0 {
		cfg.Interval = time.Hour
	}
	if cfg.UnattachedTTL <= 0 {
		cfg.UnattachedTTL = 24 * time.Hour
	}
	if cfg.BatchSize <= 0 || cfg.BatchSize > 500 {
		cfg.BatchSize = 100
	}
	return &MediaCleanupWorker{cfg: cfg}
}

func (w *MediaCleanupWorker) Start(ctx context.Context) {
	go func() {
		w.runOnce(ctx)
		ticker := time.NewTicker(w.cfg.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.runOnce(ctx)
			}
		}
	}()
}

func (w *MediaCleanupWorker) runOnce(ctx context.Context) {
	deletedBroken, deletedUnattached, err := w.cleanup(ctx)
	if err != nil {
		log.Printf("media cleanup failed: %v", err)
		return
	}
	if deletedBroken > 0 || deletedUnattached > 0 {
		log.Printf("media cleanup deleted broken=%d unattached=%d", deletedBroken, deletedUnattached)
	}
}

func (w *MediaCleanupWorker) cleanup(ctx context.Context) (int, int, error) {
	broken, err := w.cfg.MediaRepo.ListMediaWithoutStorage(ctx, nil, w.cfg.BatchSize)
	if err != nil {
		return 0, 0, err
	}
	deletedBroken := 0
	for _, media := range broken {
		if err := w.cfg.MediaRepo.DeleteMediaAttachment(ctx, nil, media.ID); err != nil {
			return deletedBroken, 0, err
		}
		deletedBroken++
	}
	remaining := w.cfg.BatchSize - deletedBroken
	if remaining <= 0 {
		return deletedBroken, 0, nil
	}
	unattached, err := w.cfg.MediaRepo.ListUnattachedMediaOlderThan(ctx, nil, time.Now().UTC().Add(-w.cfg.UnattachedTTL), remaining)
	if err != nil {
		return deletedBroken, 0, err
	}
	deletedUnattached := 0
	for _, media := range unattached {
		if err := w.cfg.MediaRepo.DeleteMediaAttachment(ctx, nil, media.ID); err != nil {
			return deletedBroken, deletedUnattached, err
		}
		if err := w.cfg.Storage.DeleteMedia(ctx, media.StoragePath); err != nil {
			return deletedBroken, deletedUnattached, err
		}
		deletedUnattached++
	}
	return deletedBroken, deletedUnattached, nil
}
