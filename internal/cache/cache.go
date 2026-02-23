package cache

import (
	"time"

	"github.com/puzpuzpuz/xsync/v4"
	"golang.org/x/sync/singleflight"
)

type CacheEntry[T any] struct {
	value     T
	fetchedAt time.Time
}

func CachedSingle[T any](
	cache *xsync.Map[string, CacheEntry[T]],
	sfg *singleflight.Group,
	key string,
	fn func() (T, error),
) (T, error) {
	return CachedSingleWithTTL(cache, sfg, key, 3*time.Second, fn)
}

func CachedSingleWithTTL[T any](
	cache *xsync.Map[string, CacheEntry[T]],
	sfg *singleflight.Group,
	key string,
	ttl time.Duration,
	fn func() (T, error),
) (T, error) {
	entry, ok := cache.Load(key)
	if ok {
		if time.Since(entry.fetchedAt) > ttl {
			go func() {
				sfg.Do(key, func() (any, error) {
					result, err := fn()
					if err == nil {
						cache.Store(key, CacheEntry[T]{value: result, fetchedAt: time.Now()})
					}
					return nil, nil
				})
			}()
		}
		return entry.value, nil
	}

	v, err, _ := sfg.Do(key, func() (any, error) {
		if e, ok := cache.Load(key); ok {
			return e, nil
		}
		res, err := fn()
		if err != nil {
			return nil, err
		}
		newEntry := CacheEntry[T]{value: res, fetchedAt: time.Now()}
		cache.Store(key, newEntry)
		return newEntry, nil
	})

	if err != nil {
		var zero T
		return zero, err
	}
	return v.(CacheEntry[T]).value, nil
}
