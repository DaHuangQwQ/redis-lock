package retry

import (
	"github.com/DaHuangQwQ/redis-lock/internal/errs"
	"sync/atomic"
	"time"
)

type FixedIntervalRetryStrategy struct {
	interval   time.Duration
	maxRetries int32
	retries    int32
}

func NewFixedIntervalRetryStrategy(interval time.Duration, maxRetries int32) (*FixedIntervalRetryStrategy, error) {
	if interval <= 0 {
		return nil, errs.NewErrInvalidIntervalValue(interval)
	}
	return &FixedIntervalRetryStrategy{
		maxRetries: maxRetries,
		interval:   interval,
	}, nil
}

func (s *FixedIntervalRetryStrategy) Next() (time.Duration, bool) {
	retries := atomic.AddInt32(&s.retries, 1)
	if s.maxRetries <= 0 || retries <= s.maxRetries {
		return s.interval, true
	}
	return 0, false
}
