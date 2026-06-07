package jobs

import (
	"context"
	"log"
	"time"

	"github.com/myfedi/gargoyle/domain/ports"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

type MediaCleanupWorkerConfig struct {
	MediaRepo           repos.MediaRepository
	Storage             ports.MediaStorage
	Interval            time.Duration
	UnattachedTTL       time.Duration
	RemoteCacheMaxBytes int64
	RemoteCacheTTL      time.Duration
	BatchSize           int
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
	if cfg.RemoteCacheTTL <= 0 {
		cfg.RemoteCacheTTL = 30 * 24 * time.Hour
	}
	if cfg.RemoteCacheMaxBytes < 0 {
		cfg.RemoteCacheMaxBytes = 0
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
	deletedBroken, deletedUnattached, prunedRemote, err := w.cleanup(ctx)
	if err != nil {
		log.Printf("media cleanup failed: %v", err)
		return
	}
	if deletedBroken > 0 || deletedUnattached > 0 || prunedRemote > 0 {
		log.Printf("media cleanup deleted broken=%d unattached=%d pruned_remote=%d", deletedBroken, deletedUnattached, prunedRemote)
	}
}

func (w *MediaCleanupWorker) cleanup(ctx context.Context) (int, int, int, error) {
	broken, err := w.cfg.MediaRepo.ListMediaWithoutStorage(ctx, nil, w.cfg.BatchSize)
	if err != nil {
		return 0, 0, 0, err
	}
	deletedBroken := 0
	for _, media := range broken {
		if err := w.cfg.MediaRepo.DeleteMediaAttachment(ctx, nil, media.ID); err != nil {
			return deletedBroken, 0, 0, err
		}
		deletedBroken++
	}
	remaining := w.cfg.BatchSize - deletedBroken
	if remaining <= 0 {
		prunedRemote, err := w.pruneRemoteCache(ctx)
		return deletedBroken, 0, prunedRemote, err
	}
	unattached, err := w.cfg.MediaRepo.ListUnattachedMediaOlderThan(ctx, nil, time.Now().UTC().Add(-w.cfg.UnattachedTTL), remaining)
	if err != nil {
		return deletedBroken, 0, 0, err
	}
	deletedUnattached := 0
	for _, media := range unattached {
		if err := w.cfg.MediaRepo.DeleteMediaAttachment(ctx, nil, media.ID); err != nil {
			return deletedBroken, deletedUnattached, 0, err
		}
		if err := w.cfg.Storage.DeleteMedia(ctx, media.StoragePath); err != nil {
			return deletedBroken, deletedUnattached, 0, err
		}
		deletedUnattached++
	}
	prunedRemote, err := w.pruneRemoteCache(ctx)
	return deletedBroken, deletedUnattached, prunedRemote, err
}

func (w *MediaCleanupWorker) pruneRemoteCache(ctx context.Context) (int, error) {
	pruned := 0
	old, err := w.cfg.MediaRepo.ListRemoteCachedMediaOlderThan(ctx, nil, time.Now().UTC().Add(-w.cfg.RemoteCacheTTL), w.cfg.BatchSize)
	if err != nil {
		return pruned, err
	}
	for _, media := range old {
		if err := w.pruneRemoteMedia(ctx, media.ID, media.StoragePath); err != nil {
			return pruned, err
		}
		pruned++
	}
	if w.cfg.RemoteCacheMaxBytes == 0 {
		return pruned, nil
	}
	total, err := w.cfg.MediaRepo.RemoteCachedMediaSize(ctx, nil)
	if err != nil {
		return pruned, err
	}
	for total > w.cfg.RemoteCacheMaxBytes && pruned < w.cfg.BatchSize {
		candidates, err := w.cfg.MediaRepo.ListRemoteCachedMediaByLastAccess(ctx, nil, 1)
		if err != nil {
			return pruned, err
		}
		if len(candidates) == 0 {
			return pruned, nil
		}
		media := candidates[0]
		if err := w.pruneRemoteMedia(ctx, media.ID, media.StoragePath); err != nil {
			return pruned, err
		}
		pruned++
		total -= media.FileSize
	}
	return pruned, nil
}

func (w *MediaCleanupWorker) pruneRemoteMedia(ctx context.Context, id, storagePath string) error {
	if err := w.cfg.MediaRepo.ClearMediaStorage(ctx, nil, id); err != nil {
		return err
	}
	return w.cfg.Storage.DeleteMedia(ctx, storagePath)
}
