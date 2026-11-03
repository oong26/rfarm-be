package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

func (s *redisStore) Get(ctx context.Context, key string) (*any, error) {
	b, err := s.rdb.Get(ctx, s.redisKey(key)).Bytes()
	if err != nil {
		return nil, fmt.Errorf("cache failed: %w", err)
	}

	var result any
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, fmt.Errorf("cache decode failed: %w", err)
	}

	return &result, nil
}

func (s *redisStore) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	b, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cache encode failed: %w", err)
	}
	return s.rdb.Set(ctx, s.redisKey(key), b, ttl).Err()
}

func (s *redisStore) Delete(ctx context.Context, key string) error {
	return s.rdb.Del(ctx, s.redisKey(key)).Err()
}
