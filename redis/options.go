package redis

import (
	"github.com/DaHuangQwQ/redis-lock/retry"
	"time"
)

type Option[T any] func(t *T)

func Apply[T any](t *T, opts ...Option[T]) {
	for _, opt := range opts {
		opt(t)
	}
}

// WithLockTimeout 指定单一一次加锁的超时时间
func WithLockTimeout(timeout time.Duration) Option[Lock] {
	return func(t *Lock) {
		t.lockTimeout = timeout
	}
}

func WithLockRetryStrategy(strategy retry.Strategy) Option[Lock] {
	return func(t *Lock) {
		t.lockRetry = strategy
	}
}
