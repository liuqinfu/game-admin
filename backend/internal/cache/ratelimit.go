package cache

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"time"
)

type FixedWindowLimiter struct {
	Runtime *Runtime
	Prefix  string
	Limit   int64
	Window  time.Duration
}

func (l FixedWindowLimiter) Allow(ctx context.Context, subject string) (bool, int64, error) {
	if l.Runtime == nil || l.Limit <= 0 || l.Window <= 0 {
		return true, 0, nil
	}
	windowStart := time.Now().UTC().Truncate(l.Window)
	key := fmt.Sprintf("%s:%d:%s", l.Prefix, windowStart.Unix(), hashSubject(subject))
	count, err := l.Runtime.Increment(ctx, key)
	if err != nil {
		return true, 0, err
	}
	if count == 1 {
		if err := l.Runtime.Expire(ctx, key, l.Window+time.Second); err != nil {
			return true, count, err
		}
	}
	return count <= l.Limit, count, nil
}

func hashSubject(subject string) string {
	sum := sha1.Sum([]byte(subject))
	return hex.EncodeToString(sum[:])
}
