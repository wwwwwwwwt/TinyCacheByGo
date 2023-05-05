/*
 * @Author: zzzzztw
 * @Date: 2023-05-04 18:53:34
 * @LastEditors: Do not edit
 * @LastEditTime: 2023-05-05 11:05:15
 * @FilePath: /TinyCacheByGo/geecache/singleflight/singleflight.go
 */
package singleflight

import (
	"sync"
	"time"
)

type call struct { // 表示正在进行中，或已经结束的请求
	wg  sync.WaitGroup
	val interface{}
	err error
}

type Group struct { //singleflight的主数据结构，管理不同key的请求call
	mu sync.Mutex
	m  map[string]*call
}

//针对一样的key， 无论DO执行多少次，fn只会执行一次
func (g *Group) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	g.mu.Lock()
	// 懒初始化
	if g.m == nil {
		g.m = make(map[string]*call)
	}

	// 如果当前map有key了，说明这个key正在执行，不需要再继续请求了等待结果就行
	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err
	}

	//否则创建一个请求，并加入mp
	c := new(call)
	c.wg.Add(1)
	g.m[key] = c

	g.mu.Unlock()
	//等待请求执行完，则Done通知所有wait的
	c.val, c.err = fn()
	c.wg.Done()
	time.Sleep(50 * time.Millisecond) //这一行逻辑有点问题，在上锁删除后，如果有此时有另外一个携程在等待上锁时，这个key的请求删除后，那个协程会认为这个key不在，继续请求
	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()
	return c.val, c.err
}
