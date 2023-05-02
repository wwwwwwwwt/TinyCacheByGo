/*
 * @Author: zzzzztw
 * @Date: 2023-05-02 22:44:01
 * @LastEditors: Do not edit
 * @LastEditTime: 2023-05-03 01:10:05
 * @FilePath: /TinyCacheByGo/geecache.go
 */
package geecache

import (
	"fmt"
	"geecache/lru"
	"log"
	"sync"
)

// 定义Getter接口，当数据不存在于缓存中时，调用Get来根据key获取到源数据
type Getter interface {
	Get(key string) ([]byte, error)
}

//给用户提供一个接口，可以自行设计函数调用源数据进缓存
type GetterFunc func(key string) ([]byte, error)

func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}

//-----------------------------------------------------------------
// 主体结构Group负责与外部交互，控制缓存储存与获取的主流程
// 一个缓存的命名空间，每个Group都有一个唯一的name，比如三个Group
// 缓存学生成绩的叫scores， 缓存学生信息的叫info， 缓存课程的叫coures
// getter 即缓存未命中时获取元数据的回调函数
// mainCache 即一开始实现的并发缓存
type Group struct {
	name      string
	getter    Getter
	mainCache cache
}

var (
	mu     sync.RWMutex
	groups = make(map[string]*Group) //存各种名字缓存空间的map
)

// 创建Group实例的方法
func NewGroup(name string, cacheBytes int64, getter Getter) *Group {
	if getter == nil {
		panic("nil Getter")
	}

	mu.Lock()
	defer mu.Unlock()
	group := &Group{
		name:      name,
		getter:    getter,
		mainCache: cache{lru: lru.New(cacheBytes, nil)},
	}
	groups[name] = group
	return group
}

// 根据名字查找对应缓存实例
// 如果名字未找到就是nil
// 使用读锁，因为不涉及变量冲突的写操作
func GetGroup(name string) *Group {
	mu.RLock()
	defer mu.RUnlock()
	g := groups[name]
	return g
}

//-------------------------------------------------------------
// 业务逻辑
/*


                    是
接收 key --> 检查是否被缓存 -----> 返回缓存值 ⑴
                |  否                         是
                |-----> 是否应当从远程节点获取 -----> 与远程节点交互 --> 返回缓存值 ⑵
                            |  否
                            |-----> 调用`回调函数`，获取值并添加到缓存 --> 返回缓存值 ⑶

*/

func (g *Group) Get(key string) (ByteView, error) {
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required")
	}

	if v, ok := g.mainCache.get(key); ok {
		log.Println("[GeeCache] hit")
		return v, nil
	}
	return g.load(key)
}

func (g *Group) load(key string) (ByteView, error) {
	return g.getLocally(key)
}

func (g *Group) getLocally(key string) (ByteView, error) {
	bytes, err := g.getter.Get(key)

	if err != nil {
		return ByteView{}, err
	}

	val := ByteView{b: cloneByte(bytes)}
	g.populateCache(key, val)
	return val, nil
}

func (g *Group) populateCache(key string, value ByteView) {
	g.mainCache.add(key, value)
}
