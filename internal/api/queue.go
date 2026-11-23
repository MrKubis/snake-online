package api

import (
	"context"
	"log/slog"
	"time"

	"github.com/MrKubis/snake-online/pkg/redisschema"
	"github.com/redis/go-redis/v9"
)

type QueueStatus struct {
	Size int `json:"size"`
}

func (s *Server) JoinQueue(ctx context.Context, userId string) (bool, error) {
	now := float64(time.Now().UnixMilli())

	pipe := s.rdb.TxPipeline()
	added := pipe.ZAddNX(ctx, redisschema.QueueKey, redis.Z{
		Score:  float64(time.Now().UnixMilli()),
		Member: userId,
	})

	pipe.ZAddNX(ctx, redisschema.HeartBeatQueueKey, redis.Z{
		Score:  now,
		Member: userId,
	})

	if _, err := pipe.Exec(ctx); err != nil {
		return false, err
	}

	return added.Val() == 1, nil
}

func (s *Server) LeaveQueue(ctx context.Context, userId string) error {
	return s.rdb.ZRem(ctx, redisschema.QueueKey, userId).Err()
}

func (s *Server) GetQueueStatus(ctx context.Context) (QueueStatus, error) {
	queueLenght, err := s.rdb.ZCard(ctx, redisschema.QueueKey).Result()
	if err != nil {
		return QueueStatus{}, err
	}

	return QueueStatus{
		Size: int(queueLenght),
	}, nil
}

// / This function deletes inactive users based on heartbeat response
func (s *Server) StartQueueReaper(ctx context.Context, every time.Duration, maxAge time.Duration) {
	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.reapStale(ctx, maxAge)
			}
		}
	}()
}

func (s *Server) reapStale(ctx context.Context, maxAge time.Duration) (int, error) {
	cutoff := time.Now().Add(-maxAge).UnixMilli()

	staleUsers, err := s.rdb.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:     redisschema.HeartBeatQueueKey,
		Start:   "-inf",
		Stop:    cutoff,
		ByScore: true,
	}).Result()

	if err != nil {
		return 0, err
	}

	if len(staleUsers) == 0 {
		return 0, nil
	}

	pipe := s.rdb.TxPipeline()
	for _, id := range staleUsers {
		// remove user from all queues
		pipe.ZRem(ctx, redisschema.QueueKey, id)
		pipe.ZRem(ctx, redisschema.HeartBeatQueueKey, id)
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return 0, nil
	}

	slog.Info("Deleted users from queue", "count", len(staleUsers))
	return len(staleUsers), nil
}

func (s *Server) Heartbeat(ctx context.Context, playerId string) error {
	return s.rdb.ZAddXX(ctx, redisschema.HeartBeatQueueKey, redis.Z{
		Score:  float64(time.Now().UnixMilli()),
		Member: playerId,
	}).Err()
}
