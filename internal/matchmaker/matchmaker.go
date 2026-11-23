package matchmaker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"uuid"

	"github.com/MrKubis/snake-online/pkg/redisschema"
	"github.com/redis/go-redis/v9"
)

type MatchMaker struct {
	rdb *redis.Client
}

const tickInterval = 500 * time.Millisecond

var pairScript = redis.NewScript(`
if redis.call('ZCARD', KEYS[1]) < 2 then
  return false
end
local p = redis.call('ZPOPMIN', KEYS[1], 2) -- {member, score, member, score}
redis.call('XADD', KEYS[2], '*', 'match_id', ARGV[1], 'p1', p[1], 'p2', p[3])
return {p[1], p[3]}
`)

func NewMatchMaker(cfg Config) (*MatchMaker, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr,
	})

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("Failed to connect to redis. Ping (%s): %w", cfg.RedisAddr, err)
	}

	slog.Info("Connected to redis", "addr", cfg.RedisAddr)

	return &MatchMaker{
		rdb: rdb,
	}, nil
}

func (m *MatchMaker) Close() error {
	return m.rdb.Close()
}

func (m *MatchMaker) RunMatchmaking(ctx context.Context) {
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Stopping matchmaking")
			return
		case <-ticker.C:
			for {
				matched, err := m.tryMatch(ctx)
				if err != nil {
					slog.Error("Error matching", "error", err)
				}
				if !matched {
					break
				}
			}
		}
	}
}

func (m *MatchMaker) tryMatch(ctx context.Context) (bool, error) {
	matchId := uuid.New().String()

	res, err := pairScript.Run(ctx, m.rdb, []string{redisschema.QueueKey, redisschema.MatchesKey},
		matchId).Result()

	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	players := res.([]any)

	slog.Info("match created", "match_id", matchId, "p1", players[0], "p2", players[1])

	return true, nil
}
