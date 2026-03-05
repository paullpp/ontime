package worker

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/ontime/server/internal/db"
	"github.com/ontime/server/internal/scheduler"
)

const (
	tickInterval  = 10 * time.Second
	workerPoolSize = 20
	claimBatch    = 50
)

// Supervisor runs the polling loop. It ticks every tickInterval, claims due
// trips from Redis, and dispatches them to a bounded goroutine pool.
type Supervisor struct {
	processor *Processor
	store     *db.Store
	scheduler *scheduler.Scheduler
}

func NewSupervisor(processor *Processor, store *db.Store, sched *scheduler.Scheduler) *Supervisor {
	return &Supervisor{processor: processor, store: store, scheduler: sched}
}

// Start blocks until ctx is cancelled. Call in a goroutine.
func (s *Supervisor) Start(ctx context.Context) {
	log.Println("[supervisor] starting polling worker")

	// On startup, re-seed Redis from Postgres in case Redis was restarted.
	if err := s.seedScheduler(ctx); err != nil {
		log.Printf("[supervisor] seed scheduler: %v", err)
	}

	sem := make(chan struct{}, workerPoolSize)
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[supervisor] shutting down")
			return
		case <-ticker.C:
			ids, err := s.scheduler.ClaimDue(ctx, time.Now(), claimBatch)
			if err != nil {
				log.Printf("[supervisor] claim due: %v", err)
				continue
			}
			if len(ids) > 0 {
				log.Printf("[supervisor] claimed %d trips for processing", len(ids))
			}
			s.dispatch(ctx, sem, ids)
		}
	}
}

func (s *Supervisor) dispatch(ctx context.Context, sem chan struct{}, ids []uuid.UUID) {
	var wg sync.WaitGroup
	for _, id := range ids {
		id := id
		sem <- struct{}{} // acquire slot
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }() // release slot
			s.processor.Process(ctx, id)
		}()
	}
	// Don't wait for wg here — let workers run in background while ticker
	// continues, but the semaphore caps concurrency.
	_ = wg
}

// seedScheduler re-adds all active trips to the Redis sorted set.
// This handles Redis restarts without losing scheduled trips.
func (s *Supervisor) seedScheduler(ctx context.Context) error {
	trips, err := s.store.GetAllActiveTrips(ctx)
	if err != nil {
		return err
	}
	log.Printf("[supervisor] seeding scheduler with %d active trips", len(trips))
	for _, trip := range trips {
		if err := s.scheduler.Schedule(ctx, trip.ID, trip.NextPollAt); err != nil {
			log.Printf("[supervisor] schedule trip %s: %v", trip.ID, err)
		}
	}
	return nil
}
