package api

import (
	"context"
	"time"

	"github.com/MrKubis/snake-online/pkg/redisschema"
	"github.com/redis/go-redis/v9"
)

type QueueStatus struct {
	Size int `json:"size"`
}

func (s *Server) JoinQueue(ctx context.Context, userId string) (bool, error) {
	added, err := s.rdb.ZAddNX(ctx, redisschema.QueueKey, redis.Z{
		Score:  float64(time.Now().UnixMilli()),
		Member: userId,
	}).Result()

	return added == 1, err
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
