package errs

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrFailedToPreemptLock = errors.New("redis-lock: 抢锁失败")
	ErrLockNotHold         = errors.New("redis-lock: 你没有持有锁")
	ErrLocked              = errors.New("redis-lock: 加锁失败，锁被人持有")
)

func NewErrInvalidIntervalValue(interval time.Duration) error {
	return fmt.Errorf("redis-lock: 无效的间隔时间 %d, 预期值应大于 0", interval)
}

func NewErrInvalidMaxIntervalValue(maxInterval, initialInterval time.Duration) error {
	return fmt.Errorf("redis-lock: 最大重试间隔的时间 [%d] 应大于等于初始重试的间隔时间 [%d] ", maxInterval, initialInterval)
}

func NewErrRetryExhausted(lastErr error) error {
	return fmt.Errorf("redis-lock: 超过最大重试次数，业务返回的最后一个 error %w", lastErr)
}
