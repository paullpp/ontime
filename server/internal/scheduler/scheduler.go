package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const pollKey = "trips:poll"

// Scheduler manages the Redis sorted set used to schedule trip polls.
// Score = Unix timestamp of next poll. Atomic Lua script prevents
// double-processing across multiple server replicas.
type Scheduler struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Scheduler {
	return &Scheduler{rdb: rdb}
}

// Schedule enqueues or updates a trip's next poll time.
func (s *Scheduler) Schedule(ctx context.Context, tripID uuid.UUID, at time.Time) error {
	return s.rdb.ZAdd(ctx, pollKey, redis.Z{
		Score:  float64(at.Unix()),
		Member: tripID.String(),
	}).Err()
}

// Unschedule removes a trip from the poll queue.
func (s *Scheduler) Unschedule(ctx context.Context, tripID uuid.UUID) error {
	return s.rdb.ZRem(ctx, pollKey, tripID.String()).Err()
}

// claimDueScript atomically fetches and removes up to `limit` members whose
// score (next poll time) is <= now. This prevents double-processing across
// replicas.
var claimDueScript = redis.NewScript(`
local now   = tonumber(ARGV[1])
local limit = tonumber(ARGV[2])
local members = redis.call('ZRANGEBYSCORE', KEYS[1], '-inf', now, 'LIMIT', 0, limit)
if #members > 0 then
    redis.call('ZREM', KEYS[1], unpack(members))
end
return members
`)

// ClaimDue atomically claims up to limit trip IDs that are due for polling.
func (s *Scheduler) ClaimDue(ctx context.Context, now time.Time, limit int) ([]uuid.UUID, error) {
	res, err := claimDueScript.Run(ctx, s.rdb, []string{pollKey},
		now.Unix(), limit).StringSlice()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("claimDue script: %w", err)
	}
	ids := make([]uuid.UUID, 0, len(res))
	for _, raw := range res {
		id, err := uuid.Parse(raw)
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}
