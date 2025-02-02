package redis

import (
	"context"
	_ "embed"
	"errors"
	"github.com/DaHuangQwQ/redis-lock/internal/errs"
	"github.com/DaHuangQwQ/redis-lock/retry"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"time"
)

var (
	//go:embed lua/unlock.lua
	luaUnlock string

	//go:embed lua/refresh.lua
	luaRefresh string

	//go:embed lua/lock.lua
	luaLock string
)

type Lock struct {
	client     redis.Cmdable
	key        string
	value      string
	valuer     func() string
	expiration time.Duration

	// 加锁的时候的单一一次超时
	lockTimeout time.Duration
	// 重试策略
	lockRetry retry.Strategy

	unlockChan chan struct{}
}

func NewLock(rdb redis.Cmdable, key string,
	expiration time.Duration, opts ...Option[Lock]) *Lock {
	strategy, _ := retry.NewExponentialBackoffRetryStrategy(time.Millisecond*100, time.Second, 10)
	l := &Lock{
		client: rdb,
		// 正常来说，访问 Redis 是一个快的事情，所以 200ms 是绰绰有余的
		// 毕竟一般来说超过 10ms 就是 Redis 上的慢查询了
		lockTimeout: time.Millisecond * 200,
		valuer: func() string {
			return uuid.New().String()
		},
		key:        key,
		expiration: expiration,
		lockRetry:  strategy,
	}
	Apply(l, opts...)
	l.value = l.valuer()
	return l
}

func (l *Lock) Lock(ctx context.Context) error {
	return retry.Retry(ctx, l.lockRetry, func() error {
		lctx, cancel := context.WithTimeout(ctx, l.lockTimeout)
		defer cancel()
		res, err := l.client.Eval(lctx,
			luaLock,
			[]string{l.key}, l.value, l.expiration).Result()
		// 加锁失败。虽然从理论上来说，此时加锁有可能是因为一些不可挽回的错误造成的
		// 但是我们这里没有区分处理
		if err != nil {
			return err
		}
		if res == "OK" {
			return nil
		}
		return errs.ErrLocked
	})
}

func (l *Lock) Unlock(ctx context.Context) error {
	res, err := l.client.Eval(ctx, luaUnlock, []string{l.key}, l.value).Int64()
	defer func() {
		//close(l.unlockChan)
		select {
		case l.unlockChan <- struct{}{}:
		default:
			// 说明没有人调用 AutoRefresh
		}
	}()
	//if err == redis.Nil {
	//	return ErrLockNotHold
	//}
	if err != nil {
		return err
	}
	if res != 1 {
		return errs.ErrLockNotHold
	}
	return nil
}

func (l *Lock) Refresh(ctx context.Context) error {
	res, err := l.client.Eval(ctx, luaRefresh, []string{l.key}, l.value, l.expiration.Seconds()).Int64()
	if err != nil {
		return err
	}
	if res != 1 {
		return errs.ErrLockNotHold
	}
	return nil
}

func (l *Lock) AutoRefresh(interval time.Duration, timeout time.Duration) error {
	timeoutChan := make(chan struct{}, 1)
	// 间隔多久续约一次
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-ticker.C:
			// 刷新的超时时间怎么设置
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			// 出现了 error 了怎么办？
			err := l.Refresh(ctx)
			cancel()
			if errors.Is(err, context.DeadlineExceeded) {
				timeoutChan <- struct{}{}
				continue
			}
			if err != nil {
				return err
			}
		case <-timeoutChan:
			// 刷新的超时时间怎么设置
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			// 出现了 error 了怎么办？
			err := l.Refresh(ctx)
			cancel()
			if errors.Is(err, context.DeadlineExceeded) {
				timeoutChan <- struct{}{}
				continue
			}
			if err != nil {
				return err
			}

		case <-l.unlockChan:
			return nil
		}
	}
}
