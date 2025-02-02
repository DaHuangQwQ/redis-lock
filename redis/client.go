package redis

import (
	"context"
	"errors"
	"fmt"
	"github.com/DaHuangQwQ/redis-lock/internal/errs"
	"github.com/DaHuangQwQ/redis-lock/retry"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
	"time"
)

// Client 就是对 redis.Cmdable 的二次封装
type Client struct {
	client redis.Cmdable
	//valGenerator func() string
	g singleflight.Group
}

func NewClient(client redis.Cmdable) *Client {
	return &Client{
		client: client,
	}
}

func (c *Client) SingleflightLock(ctx context.Context,
	key string,
	expiration time.Duration,
	timeout time.Duration, retry retry.Strategy) (*Lock, error) {
	for {
		flag := false
		resCh := c.g.DoChan(key, func() (interface{}, error) {
			flag = true
			return c.Lock(ctx, key, expiration, timeout, retry)
		})
		select {
		case res := <-resCh:
			if flag {
				c.g.Forget(key)
				if res.Err != nil {
					return nil, res.Err
				}
				return res.Val.(*Lock), nil
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (c *Client) Lock(ctx context.Context,
	key string,
	expiration time.Duration,
	timeout time.Duration, retry retry.Strategy) (*Lock, error) {
	var timer *time.Timer
	val := uuid.New().String()
	//val := c.valGenerator()
	for {
		// 在这里重试
		lctx, cancel := context.WithTimeout(ctx, timeout)
		res, err := c.client.Eval(lctx, luaLock, []string{key}, val, expiration.Seconds()).Result()
		cancel()
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}

		if res == "OK" {
			return &Lock{
				client:     c.client,
				key:        key,
				value:      val,
				expiration: expiration,
				unlockChan: make(chan struct{}, 1),
			}, nil
		}

		interval, ok := retry.Next()
		if !ok {
			return nil, fmt.Errorf("redis-lock: 超出重试限制, %w", errs.ErrFailedToPreemptLock)
		}
		if timer == nil {
			timer = time.NewTimer(interval)
		} else {
			timer.Reset(interval)
		}
		select {
		case <-timer.C:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (c *Client) TryLock(ctx context.Context,
	key string,
	expiration time.Duration) (*Lock, error) {
	val := uuid.New().String()
	ok, err := c.client.SetNX(ctx, key, val, expiration).Result()
	if err != nil {
		return nil, err
	}
	if !ok {
		// 代表的是别人抢到了锁
		return nil, errs.ErrFailedToPreemptLock
	}
	return &Lock{
		client:     c.client,
		key:        key,
		value:      val,
		expiration: expiration,
		unlockChan: make(chan struct{}, 1),
	}, nil
}
