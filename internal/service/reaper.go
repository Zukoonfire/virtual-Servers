package service

import (
	"context"
	"log"
	"time"
	"virtualservers/internal/repository"
)

func StartIdleReaper(ctx context.Context, store *repository.Store, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("idle reaper stopped")
			return
		case <-ticker.C:
			reaped, err := store.ReapIdleServers(ctx)
			if err != nil {
				log.Printf("idle reaper error:%v", err)
			} else if reaped > 0 {
				log.Printf("idle reaper terminated %d servers", reaped)
			}
		}

	}
}
