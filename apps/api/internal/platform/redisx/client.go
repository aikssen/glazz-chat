package redisx

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/aikssen/glazz-chat/apps/api/internal/platform/config"
)

var ErrNotFound = errors.New("redis value not found")

type Client struct {
	client *redis.Client
	prefix string
}

func Open(ctx context.Context, cfg config.Redis) (*Client, error) {
	options, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse Redis configuration: %w", err)
	}
	client := redis.NewClient(options)
	healthCtx, cancel := context.WithTimeout(ctx, cfg.HealthTimeout)
	defer cancel()
	if err := client.Ping(healthCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping Redis: %w", err)
	}
	return &Client{client: client, prefix: cfg.Prefix}, nil
}

func (c *Client) Close() error {
	return c.client.Close()
}

func (c *Client) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

func (c *Client) key(parts ...string) string {
	clean := make([]string, 0, len(parts)+1)
	clean = append(clean, c.prefix)
	for _, part := range parts {
		clean = append(clean, strings.ReplaceAll(part, ":", "_"))
	}
	return strings.Join(clean, ":")
}

func (c *Client) Put(ctx context.Context, namespace, id, value string, ttl time.Duration) error {
	if ttl <= 0 {
		return errors.New("Redis Put TTL must be positive")
	}
	return c.client.Set(ctx, c.key(namespace, id), value, ttl).Err()
}

func (c *Client) Get(ctx context.Context, namespace, id string) (string, error) {
	value, err := c.client.Get(ctx, c.key(namespace, id)).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read Redis value: %w", err)
	}
	return value, nil
}

func (c *Client) Take(ctx context.Context, namespace, id string) (string, error) {
	value, err := c.client.GetDel(ctx, c.key(namespace, id)).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("consume Redis value: %w", err)
	}
	return value, nil
}

func (c *Client) Delete(ctx context.Context, namespace, id string) error {
	return c.client.Del(ctx, c.key(namespace, id)).Err()
}

type RateLimitResult struct {
	Allowed   bool
	Used      int64
	Remaining int64
	ResetIn   time.Duration
}

var rateLimitScript = redis.NewScript(`
local count = redis.call("INCRBY", KEYS[1], ARGV[1])
if count == tonumber(ARGV[1]) then
  redis.call("PEXPIRE", KEYS[1], ARGV[3])
end
local ttl = redis.call("PTTL", KEYS[1])
local limit = tonumber(ARGV[2])
return {count <= limit and 1 or 0, count, math.max(limit - count, 0), ttl}
`)

func (c *Client) AddRateUsage(
	ctx context.Context,
	bucket string,
	amount, limit int64,
	window time.Duration,
) (RateLimitResult, error) {
	if amount <= 0 || limit <= 0 || window <= 0 {
		return RateLimitResult{}, errors.New("rate usage amount, limit, and window must be positive")
	}
	values, err := rateLimitScript.Run(
		ctx,
		c.client,
		[]string{c.key("rate", bucket)},
		amount,
		limit,
		window.Milliseconds(),
	).Int64Slice()
	if err != nil {
		return RateLimitResult{}, fmt.Errorf("apply Redis rate usage: %w", err)
	}
	return RateLimitResult{
		Allowed:   values[0] == 1,
		Used:      values[1],
		Remaining: values[2],
		ResetIn:   time.Duration(values[3]) * time.Millisecond,
	}, nil
}

func (c *Client) AcquireLease(
	ctx context.Context,
	namespace, id, owner string,
	ttl time.Duration,
) (bool, error) {
	if owner == "" || ttl <= 0 {
		return false, errors.New("lease owner and positive TTL are required")
	}
	acquired, err := c.client.SetNX(ctx, c.key("lease", namespace, id), owner, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("acquire Redis lease: %w", err)
	}
	return acquired, nil
}

var releaseLeaseScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0
`)

func (c *Client) ReleaseLease(ctx context.Context, namespace, id, owner string) (bool, error) {
	released, err := releaseLeaseScript.Run(
		ctx,
		c.client,
		[]string{c.key("lease", namespace, id)},
		owner,
	).Int()
	if err != nil {
		return false, fmt.Errorf("release Redis lease: %w", err)
	}
	return released == 1, nil
}

func (c *Client) Publish(ctx context.Context, topic, payload string) error {
	return c.client.Publish(ctx, c.key("topic", topic), payload).Err()
}

func (c *Client) Subscribe(ctx context.Context, topic string) (*redis.PubSub, error) {
	subscription := c.client.Subscribe(ctx, c.key("topic", topic))
	if _, err := subscription.Receive(ctx); err != nil {
		_ = subscription.Close()
		return nil, fmt.Errorf("subscribe Redis topic: %w", err)
	}
	return subscription, nil
}

func (c *Client) NextSequence(
	ctx context.Context,
	namespace, id string,
	ttl time.Duration,
) (int64, error) {
	if ttl <= 0 {
		return 0, errors.New("sequence TTL must be positive")
	}
	key := c.key("sequence", namespace, id)
	sequence, err := c.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("increment Redis sequence: %w", err)
	}
	if err := c.client.Expire(ctx, key, ttl).Err(); err != nil {
		return 0, fmt.Errorf("expire Redis sequence: %w", err)
	}
	return sequence, nil
}

func (c *Client) CurrentSequence(
	ctx context.Context,
	namespace, id string,
) (int64, error) {
	sequence, err := c.client.Get(ctx, c.key("sequence", namespace, id)).Int64()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read Redis sequence: %w", err)
	}
	return sequence, nil
}

func (c *Client) AppendReplay(
	ctx context.Context,
	namespace, id string,
	sequence int64,
	payload string,
	limit int64,
	ttl time.Duration,
) error {
	if sequence <= 0 || limit <= 0 || ttl <= 0 {
		return errors.New("positive sequence, replay limit, and TTL are required")
	}
	key := c.key("replay", namespace, id)
	pipe := c.client.TxPipeline()
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(sequence), Member: payload})
	pipe.ZRemRangeByRank(ctx, key, 0, -(limit + 1))
	pipe.Expire(ctx, key, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("append Redis replay event: %w", err)
	}
	return nil
}

func (c *Client) ReplayAfter(
	ctx context.Context,
	namespace, id string,
	sequence int64,
	limit int64,
) ([]string, error) {
	if sequence < 0 || limit <= 0 {
		return nil, errors.New("non-negative sequence and positive limit are required")
	}
	values, err := c.client.ZRangeByScore(ctx, c.key("replay", namespace, id), &redis.ZRangeBy{
		Min: strconv.FormatInt(sequence+1, 10), Max: "+inf", Count: limit,
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("read Redis replay events: %w", err)
	}
	return values, nil
}
