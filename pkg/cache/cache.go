package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/oong26/rfarm-be/internal/configs"
)

const keyPrefix = "rfarm:"

type redisStore struct{ rdb *redis.Client }

// Store is the Redis-backed storage interface.
type Store interface {
	Get(ctx context.Context, key string) (*any, error)
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

// NewClient constructs a *redis.Client from configs.Redis.
// It applies connection pooling, dial/read/write timeouts, and verifies
// connectivity via PING. It does not manage sessions.
func NewClient(cfg configs.Redis) (*redis.Client, error) {
	dialTimeout, err := time.ParseDuration(cfg.DialTimeout)
	if err != nil {
		return nil, fmt.Errorf("cache: invalid dialTimeout %q: %w", cfg.DialTimeout, err)
	}
	readTimeout, err := time.ParseDuration(cfg.ReadTimeout)
	if err != nil {
		return nil, fmt.Errorf("cache: invalid readTimeout %q: %w", cfg.ReadTimeout, err)
	}
	writeTimeout, err := time.ParseDuration(cfg.WriteTimeout)
	if err != nil {
		return nil, fmt.Errorf("cache: invalid writeTimeout %q: %w", cfg.WriteTimeout, err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           int(cfg.DB),
		DialTimeout:  dialTimeout,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		PoolSize:     int(cfg.PoolSize),
	})

	ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("cache: redis health check failed: %w", err)
	}

	return rdb, nil
}

// NewRedisStore wraps an existing *redis.Client as a Store.
func NewRedisStore(rdb *redis.Client) Store {
	if rdb == nil {
		return nil
	}
	return &redisStore{rdb: rdb}
}

func (s *redisStore) redisKey(key string) string { return keyPrefix + key }