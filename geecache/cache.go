/*
 * @Author: zzzzztw
 * @Date: 2023-05-02 20:11:28
 * @LastEditors: Do not edit
 * @LastEditTime: 2023-05-03 00:13:36
 * @FilePath: /TinyCacheByGo/cache.go
 */
package geecache

import (
	"geecache/lru"
	"sync"
)

type cache struct {
	mu         sync.Mutex
	lru        *lru.Cache
	cacheBytes int64
}

//-----------------------------------------------------------------
// 使用锁来包装裸的lru结构实现并发
func (c *cache) add(key string, value ByteView) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.lru == nil {
		c.lru = lru.New(c.cacheBytes, nil) // 延迟初始化，第一次使用add时才会创建一个实例对象
	}
	c.lru.Add(key, value)
}

func (c *cache) get(key string) (value ByteView, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.lru == nil {
		return
	}

	if v, ok := c.lru.Get(key); ok {
		return v.(ByteView), ok
	}
	return
}
