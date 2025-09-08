package service

import (
	"context"
	"log"
	"time"

	"virtualservers/internal/repository"
)

// StartBillingDaemon runs in background untill ctx is cancelled
func StartBillingDaemon(ctx context.Context, store *repository.Store, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Println("billing daemon stopped")
			return
		case <-ticker.C:
			updated, err := store.AccrueBilling(ctx)
			if err != nil {
				log.Printf("billing daemon error:%v", err)
			} else if updated > 0 {
				log.Printf("billing daemon updated %d servers", updated)
			}
		}
	}
}
