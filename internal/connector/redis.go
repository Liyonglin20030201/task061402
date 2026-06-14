package connector

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/Liyonglin20030201/task061402/internal/config"
)

type RedisConnector struct {
	target config.Target
	client *redis.Client
}

func NewRedis(target config.Target) *RedisConnector {
	return &RedisConnector{target: target}
}

func (r *RedisConnector) Connect(ctx context.Context) error {
	r.client = redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", r.target.Host, r.target.Port),
		Password:     r.target.Password,
		DB:           r.target.DB,
		DialTimeout:  10 * time.Second,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	})

	if err := r.client.Ping(ctx).Err(); err != nil {
		r.client.Close()
		return fmt.Errorf("failed to ping Redis %s: %w", r.target.Name, err)
	}

	return nil
}

func (r *RedisConnector) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}

func (r *RedisConnector) Type() string { return "redis" }
func (r *RedisConnector) Name() string { return r.target.Name }

func (r *RedisConnector) Ping(ctx context.Context) error {
	if r.client == nil {
		return fmt.Errorf("not connected")
	}
	return r.client.Ping(ctx).Err()
}

func (r *RedisConnector) Client() *redis.Client { return r.client }

func (r *RedisConnector) Do(ctx context.Context, args ...interface{}) (interface{}, error) {
	if r.client == nil {
		return nil, fmt.Errorf("not connected")
	}
	cmd := r.client.Do(ctx, args...)
	return cmd.Result()
}

func (r *RedisConnector) Info(ctx context.Context, section string) (string, error) {
	if r.client == nil {
		return "", fmt.Errorf("not connected")
	}
	return r.client.Info(ctx, section).Result()
}

func (r *RedisConnector) SlowLogGet(ctx context.Context, count int64) ([]SlowLogEntry, error) {
	if r.client == nil {
		return nil, fmt.Errorf("not connected")
	}

	result, err := r.client.SlowLogGet(ctx, count).Result()
	if err != nil {
		return nil, err
	}

	var entries []SlowLogEntry
	for _, item := range result {
		entry := SlowLogEntry{
			ID:        item.ID,
			Timestamp: item.Time.Unix(),
			Duration:  item.Duration.Microseconds(),
			ClientAddr: item.ClientAddr,
			ClientName: item.ClientName,
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func (r *RedisConnector) DBSize(ctx context.Context) (int64, error) {
	if r.client == nil {
		return 0, fmt.Errorf("not connected")
	}
	return r.client.DBSize(ctx).Result()
}

func (r *RedisConnector) MemoryUsage(ctx context.Context) (int64, error) {
	info, err := r.Info(ctx, "memory")
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(info, "\r\n") {
		if strings.HasPrefix(line, "used_memory:") {
			val := strings.TrimPrefix(line, "used_memory:")
			return strconv.ParseInt(val, 10, 64)
		}
	}
	return 0, fmt.Errorf("memory info not found")
}
