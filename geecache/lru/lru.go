/*
 * @Author: zzzzztw
 * @Date: 2023-05-02 14:30:39
 * @LastEditors: Do not edit
 * @LastEditTime: 2023-05-02 15:58:15
 * @FilePath: /geecache/lru/lru.go
 */
package lru

import "container/list"

// Cache时裸的Lru cache，并发时不安全
type Cache struct {
	maxBytes  int64 //允许使用的最大内存
	nbytes    int64 //当前已经使用的内存
	ll        *list.List
	cache     map[string]*list.Element
	OnEvicted func(key string, value Value)
}

// 双向链表的节点类型
type entry struct {
	key   string
	value Value
}

// 为了通用性，允许值Value接口是任意类型，该接口只包含一个方法，用于返回值所占内存大小
type Value interface {
	Len() int
}

func New(maxBytes int64, onEvicted func(string, Value)) *Cache {
	return &Cache{
		maxBytes:  maxBytes,
		ll:        list.New(),
		cache:     make(map[string]*list.Element),
		OnEvicted: onEvicted,
	}
}

func (c *Cache) Get(key string) (value Value, ok bool) {
	if ele, ok := c.cache[key]; ok {
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*entry)
		return kv.value, true
	}
	return
}

func (c *Cache) RemoveOldest() {
	ele := c.ll.Back()
	if ele != nil {
		c.ll.Remove(ele)
		kv := ele.Value.(*entry)
		delete(c.cache, kv.key)
		c.nbytes -= int64(len(kv.key)) + int64(kv.value.Len())
		if c.OnEvicted != nil {
			c.OnEvicted(kv.key, kv.value)
		}
	}
}

func (c *Cache) Add(key string, value Value) {
	if ele, ok := c.cache[key]; ok {
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*entry)
		c.nbytes += int64(value.Len()) - int64(kv.value.Len())
		kv.value = value
	} else {
		ele := c.ll.PushFront(&entry{key: key, value: value})
		c.cache[key] = ele
		c.nbytes += int64(len(key)) + int64(value.Len())
	}

	for c.maxBytes != 0 && c.nbytes > c.maxBytes {
		c.RemoveOldest()
	}
}

func (c *Cache) Len() int {
	return c.ll.Len()
}
