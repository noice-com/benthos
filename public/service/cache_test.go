package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Jeffail/benthos/v3/lib/metrics"
	"github.com/Jeffail/benthos/v3/lib/types"
	"github.com/stretchr/testify/assert"
)

type testCacheItem struct {
	b   []byte
	ttl *time.Duration
}

type closableCache struct {
	m      map[string]testCacheItem
	err    error
	closed bool
}

func (c *closableCache) Get(ctx context.Context, key string) ([]byte, error) {
	if c.err != nil {
		return nil, c.err
	}
	i, ok := c.m[key]
	if !ok {
		return nil, types.ErrKeyNotFound
	}
	return i.b, nil
}

func (c *closableCache) Set(ctx context.Context, key string, value []byte, ttl *time.Duration) error {
	if c.err != nil {
		return c.err
	}
	c.m[key] = testCacheItem{
		b: value, ttl: ttl,
	}
	return nil
}

func (c *closableCache) Add(ctx context.Context, key string, value []byte, ttl *time.Duration) error {
	if c.err != nil {
		return c.err
	}
	if _, ok := c.m[key]; ok {
		return types.ErrKeyAlreadyExists
	}
	c.m[key] = testCacheItem{
		b: value, ttl: ttl,
	}
	return nil

}

func (c *closableCache) Delete(ctx context.Context, key string) error {
	if c.err != nil {
		return c.err
	}
	delete(c.m, key)
	return nil
}

func (c *closableCache) Close(ctx context.Context) error {
	c.closed = true
	return nil
}

type closableCacheMulti struct {
	*closableCache

	multiItems map[string]testCacheItem
}

func (c *closableCacheMulti) SetMulti(ctx context.Context, keyValues ...CacheItem) error {
	if c.closableCache.err != nil {
		return c.closableCache.err
	}
	for _, kv := range keyValues {
		c.multiItems[kv.Key] = testCacheItem{
			b:   kv.Value,
			ttl: kv.TTL,
		}
	}
	return nil
}

func TestCacheAirGapShutdown(t *testing.T) {
	rl := &closableCache{}
	agrl := newAirGapCache(rl, metrics.Noop())

	err := agrl.WaitForClose(time.Millisecond * 5)
	assert.EqualError(t, err, "action timed out")
	assert.False(t, rl.closed)

	agrl.CloseAsync()
	err = agrl.WaitForClose(time.Millisecond * 5)
	assert.NoError(t, err)
	assert.True(t, rl.closed)
}

func TestCacheAirGapGet(t *testing.T) {
	rl := &closableCache{
		m: map[string]testCacheItem{
			"foo": {
				b: []byte("bar"),
			},
		},
	}
	agrl := newAirGapCache(rl, metrics.Noop())

	b, err := agrl.Get("foo")
	assert.NoError(t, err)
	assert.Equal(t, "bar", string(b))

	_, err = agrl.Get("not exist")
	assert.Equal(t, err, ErrKeyNotFound)
	assert.EqualError(t, err, "key does not exist")
}

func TestCacheAirGapSet(t *testing.T) {
	rl := &closableCache{
		m: map[string]testCacheItem{},
	}
	agrl := newAirGapCache(rl, metrics.Noop())

	err := agrl.Set("foo", []byte("bar"))
	assert.NoError(t, err)
	assert.Equal(t, map[string]testCacheItem{
		"foo": {
			b:   []byte("bar"),
			ttl: nil,
		},
	}, rl.m)

	err = agrl.Set("foo", []byte("baz"))
	assert.NoError(t, err)
	assert.Equal(t, map[string]testCacheItem{
		"foo": {
			b:   []byte("baz"),
			ttl: nil,
		},
	}, rl.m)
}

func TestCacheAirGapSetMulti(t *testing.T) {
	rl := &closableCache{
		m: map[string]testCacheItem{},
	}
	agrl := newAirGapCache(rl, metrics.Noop())

	err := agrl.SetMulti(map[string][]byte{
		"first":  []byte("bar"),
		"second": []byte("baz"),
	})
	assert.NoError(t, err)
	assert.Equal(t, map[string]testCacheItem{
		"first": {
			b:   []byte("bar"),
			ttl: nil,
		},
		"second": {
			b:   []byte("baz"),
			ttl: nil,
		},
	}, rl.m)
}

func TestCacheAirGapSetMultiWithTTL(t *testing.T) {
	rl := &closableCache{
		m: map[string]testCacheItem{},
	}
	agrl := newAirGapCache(rl, metrics.Noop()).(types.CacheWithTTL)

	ttl1, ttl2 := time.Second, time.Millisecond

	err := agrl.SetMultiWithTTL(map[string]types.CacheTTLItem{
		"first": {
			Value: []byte("bar"),
			TTL:   &ttl1,
		},
		"second": {
			Value: []byte("baz"),
			TTL:   &ttl2,
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, map[string]testCacheItem{
		"first": {
			b:   []byte("bar"),
			ttl: &ttl1,
		},
		"second": {
			b:   []byte("baz"),
			ttl: &ttl2,
		},
	}, rl.m)
}

func TestCacheAirGapSetMultiWithTTLPassthrough(t *testing.T) {
	rl := &closableCacheMulti{
		closableCache: &closableCache{
			m: map[string]testCacheItem{},
		},
		multiItems: map[string]testCacheItem{},
	}
	agrl := newAirGapCache(rl, metrics.Noop()).(types.CacheWithTTL)

	ttl1, ttl2 := time.Second, time.Millisecond

	err := agrl.SetMultiWithTTL(map[string]types.CacheTTLItem{
		"first": {
			Value: []byte("bar"),
			TTL:   &ttl1,
		},
		"second": {
			Value: []byte("baz"),
			TTL:   &ttl2,
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, map[string]testCacheItem{}, rl.m)
	assert.Equal(t, map[string]testCacheItem{
		"first": {
			b:   []byte("bar"),
			ttl: &ttl1,
		},
		"second": {
			b:   []byte("baz"),
			ttl: &ttl2,
		},
	}, rl.multiItems)
}

func TestCacheAirGapSetWithTTL(t *testing.T) {
	rl := &closableCache{
		m: map[string]testCacheItem{},
	}
	agrl := newAirGapCache(rl, metrics.Noop()).(types.CacheWithTTL)

	ttl1, ttl2 := time.Second, time.Millisecond
	err := agrl.SetWithTTL("foo", []byte("bar"), &ttl1)
	assert.NoError(t, err)
	assert.Equal(t, map[string]testCacheItem{
		"foo": {
			b:   []byte("bar"),
			ttl: &ttl1,
		},
	}, rl.m)

	err = agrl.SetWithTTL("foo", []byte("baz"), &ttl2)
	assert.NoError(t, err)
	assert.Equal(t, map[string]testCacheItem{
		"foo": {
			b:   []byte("baz"),
			ttl: &ttl2,
		},
	}, rl.m)
}

func TestCacheAirGapAdd(t *testing.T) {
	rl := &closableCache{
		m: map[string]testCacheItem{},
	}
	agrl := newAirGapCache(rl, metrics.Noop())

	err := agrl.Add("foo", []byte("bar"))
	assert.NoError(t, err)
	assert.Equal(t, map[string]testCacheItem{
		"foo": {
			b:   []byte("bar"),
			ttl: nil,
		},
	}, rl.m)

	err = agrl.Add("foo", []byte("baz"))
	assert.Equal(t, err, ErrKeyAlreadyExists)
	assert.EqualError(t, err, "key already exists")
}

func TestCacheAirGapAddWithTTL(t *testing.T) {
	rl := &closableCache{
		m: map[string]testCacheItem{},
	}
	agrl := newAirGapCache(rl, metrics.Noop()).(types.CacheWithTTL)

	ttl := time.Second
	err := agrl.AddWithTTL("foo", []byte("bar"), &ttl)
	assert.NoError(t, err)
	assert.Equal(t, map[string]testCacheItem{
		"foo": {
			b:   []byte("bar"),
			ttl: &ttl,
		},
	}, rl.m)

	err = agrl.AddWithTTL("foo", []byte("baz"), nil)
	assert.Equal(t, err, ErrKeyAlreadyExists)
	assert.EqualError(t, err, "key already exists")
}

func TestCacheAirGapDelete(t *testing.T) {
	rl := &closableCache{
		m: map[string]testCacheItem{
			"foo": {
				b: []byte("bar"),
			},
		},
	}
	agrl := newAirGapCache(rl, metrics.Noop())

	err := agrl.Delete("foo")
	assert.NoError(t, err)
	assert.Equal(t, map[string]testCacheItem{}, rl.m)
}

type closableCacheType struct {
	m      map[string]testCacheItem
	err    error
	closed bool
}

func (c *closableCacheType) Get(key string) ([]byte, error) {
	if c.err != nil {
		return nil, c.err
	}
	i, ok := c.m[key]
	if !ok {
		return nil, types.ErrKeyNotFound
	}
	return i.b, nil
}

func (c *closableCacheType) Set(key string, value []byte) error {
	if c.err != nil {
		return c.err
	}
	c.m[key] = testCacheItem{b: value}
	return nil
}
func (c *closableCacheType) SetWithTTL(key string, value []byte, ttl *time.Duration) error {
	if c.err != nil {
		return c.err
	}
	c.m[key] = testCacheItem{
		b: value, ttl: ttl,
	}
	return nil
}

func (c *closableCacheType) SetMulti(map[string][]byte) error {
	return errors.New("not implemented")
}

func (c *closableCacheType) SetMultiWithTTL(items map[string]types.CacheTTLItem) error {
	return errors.New("not implemented")
}

func (c *closableCacheType) Add(key string, value []byte) error {
	if c.err != nil {
		return c.err
	}
	if _, ok := c.m[key]; ok {
		return types.ErrKeyAlreadyExists
	}
	c.m[key] = testCacheItem{b: value}
	return nil

}

func (c *closableCacheType) AddWithTTL(key string, value []byte, ttl *time.Duration) error {
	if c.err != nil {
		return c.err
	}
	if _, ok := c.m[key]; ok {
		return types.ErrKeyAlreadyExists
	}
	c.m[key] = testCacheItem{
		b: value, ttl: ttl,
	}
	return nil

}

func (c *closableCacheType) Delete(key string) error {
	if c.err != nil {
		return c.err
	}
	delete(c.m, key)
	return nil
}

func (c *closableCacheType) CloseAsync() {
	c.closed = true
}

func (c *closableCacheType) WaitForClose(t time.Duration) error {
	return nil
}

func TestCacheReverseAirGapShutdown(t *testing.T) {
	rl := &closableCacheType{}
	agrl := newReverseAirGapCache(rl)

	err := agrl.Close(context.Background())
	assert.NoError(t, err)
	assert.True(t, rl.closed)
}

func TestCacheReverseAirGapGet(t *testing.T) {
	rl := &closableCacheType{
		m: map[string]testCacheItem{
			"foo": {
				b: []byte("bar"),
			},
		},
	}
	agrl := newReverseAirGapCache(rl)

	b, err := agrl.Get(context.Background(), "foo")
	assert.NoError(t, err)
	assert.Equal(t, "bar", string(b))

	_, err = agrl.Get(context.Background(), "not exist")
	assert.Equal(t, err, ErrKeyNotFound)
	assert.EqualError(t, err, "key does not exist")
}

func TestCacheReverseAirGapSet(t *testing.T) {
	rl := &closableCacheType{
		m: map[string]testCacheItem{},
	}
	agrl := newReverseAirGapCache(rl)

	err := agrl.Set(context.Background(), "foo", []byte("bar"), nil)
	assert.NoError(t, err)
	assert.Equal(t, map[string]testCacheItem{
		"foo": {
			b:   []byte("bar"),
			ttl: nil,
		},
	}, rl.m)

	err = agrl.Set(context.Background(), "foo", []byte("baz"), nil)
	assert.NoError(t, err)
	assert.Equal(t, map[string]testCacheItem{
		"foo": {
			b:   []byte("baz"),
			ttl: nil,
		},
	}, rl.m)
}

func TestCacheReverseAirGapSetWithTTL(t *testing.T) {
	rl := &closableCacheType{
		m: map[string]testCacheItem{},
	}
	agrl := newReverseAirGapCache(rl)

	ttl1, ttl2 := time.Second, time.Millisecond
	err := agrl.Set(context.Background(), "foo", []byte("bar"), &ttl1)
	assert.NoError(t, err)
	assert.Equal(t, map[string]testCacheItem{
		"foo": {
			b:   []byte("bar"),
			ttl: &ttl1,
		},
	}, rl.m)

	err = agrl.Set(context.Background(), "foo", []byte("baz"), &ttl2)
	assert.NoError(t, err)
	assert.Equal(t, map[string]testCacheItem{
		"foo": {
			b:   []byte("baz"),
			ttl: &ttl2,
		},
	}, rl.m)
}

func TestCacheReverseAirGapAdd(t *testing.T) {
	rl := &closableCacheType{
		m: map[string]testCacheItem{},
	}
	agrl := newReverseAirGapCache(rl)

	err := agrl.Add(context.Background(), "foo", []byte("bar"), nil)
	assert.NoError(t, err)
	assert.Equal(t, map[string]testCacheItem{
		"foo": {
			b:   []byte("bar"),
			ttl: nil,
		},
	}, rl.m)

	err = agrl.Add(context.Background(), "foo", []byte("baz"), nil)
	assert.Equal(t, err, ErrKeyAlreadyExists)
	assert.EqualError(t, err, "key already exists")
}

func TestCacheReverseAirGapAddWithTTL(t *testing.T) {
	rl := &closableCacheType{
		m: map[string]testCacheItem{},
	}
	agrl := newReverseAirGapCache(rl)

	ttl := time.Second
	err := agrl.Add(context.Background(), "foo", []byte("bar"), &ttl)
	assert.NoError(t, err)
	assert.Equal(t, map[string]testCacheItem{
		"foo": {
			b:   []byte("bar"),
			ttl: &ttl,
		},
	}, rl.m)

	err = agrl.Add(context.Background(), "foo", []byte("baz"), nil)
	assert.Equal(t, err, ErrKeyAlreadyExists)
	assert.EqualError(t, err, "key already exists")
}

func TestCacheReverseAirGapDelete(t *testing.T) {
	rl := &closableCacheType{
		m: map[string]testCacheItem{
			"foo": {
				b: []byte("bar"),
			},
		},
	}
	agrl := newReverseAirGapCache(rl)

	err := agrl.Delete(context.Background(), "foo")
	assert.NoError(t, err)
	assert.Equal(t, map[string]testCacheItem{}, rl.m)
}
