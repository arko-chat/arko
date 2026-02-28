package cache

import (
	"time"

	"github.com/puzpuzpuz/xsync/v4"
	"golang.org/x/sync/singleflight"
)

type entry[T any] struct {
	value     T
	fetchedAt time.Time
}

type Cache[T any] struct {
	store *xsync.Map[string, entry[T]]
	sfg   singleflight.Group
	ttl   time.Duration
}

func New[T any](ttl time.Duration) *Cache[T] {
	return &Cache[T]{
		store: xsync.NewMap[string, entry[T]](),
		ttl:   ttl,
	}
}

func NewDefault[T any]() *Cache[T] {
	return New[T](3 * time.Second)
}

func (c *Cache[T]) Get(key string, fn func() (T, error)) (T, error) {
	if e, ok := c.store.Load(key); ok {
		if time.Since(e.fetchedAt) > c.ttl {
			go func() {
				c.sfg.Do(key, func() (any, error) {
					res, err := fn()
					if err == nil {
						c.store.Store(key, entry[T]{value: res, fetchedAt: time.Now()})
					}
					return nil, nil
				})
			}()
		}
		return e.value, nil
	}

	v, err, _ := c.sfg.Do(key, func() (any, error) {
		if e, ok := c.store.Load(key); ok {
			return e, nil
		}
		res, err := fn()
		if err != nil {
			return nil, err
		}
		newEntry := entry[T]{value: res, fetchedAt: time.Now()}
		c.store.Store(key, newEntry)
		return newEntry, nil
	})

	if err != nil {
		var zero T
		return zero, err
	}
	return v.(entry[T]).value, nil
}

func (c *Cache[T]) Invalidate(key string) {
	c.store.Delete(key)
}

func (c *Cache[T]) Clear() {
	c.store.Clear()
}
