<!--
 * @Author: zzzzztw
 * @Date: 2023-05-02 14:29:18
 * @LastEditors: Do not edit
 * @LastEditTime: 2023-05-06 11:08:10
 * @FilePath: /TinyCacheByGo/README.md
-->
# 基于Go的简易分布式缓存框架🚀

仿照Go 语言的groupcache，进行开发

## Prerequisites

- Golang 1.18 or later
- gRPC-go v1.55.0 or later
- protobuf v1.30.0 or later

---

### 为什么要用分布式缓存

- 为什么要用缓存：第一次请求时将一些耗时操作的结果暂存，以后遇到相同的请求，直接返回暂存的数据。比如微博的点赞的数量，不可能每个人每次访问，都从数据库中查找所有点赞的记录再统计，数据库的操作是很耗时的，很难支持那么大的流量，所以一般点赞这类数据是缓存在 Redis 服务集群中的。
  
- 为什么要用分布式系统：单台计算机的资源是有限的，计算、存储等都是有限的。随着业务量和访问量的增加，单台机器很容易遇到瓶颈。如果利用多台计算机的资源，并行处理提高性能就要缓存应用能够支持分布式，这称为水平扩展(scale horizontally)。与水平扩展相对应的是垂直扩展(scale vertically)，即通过增加单个节点的计算、存储、带宽等，来提高系统的性能，硬件的成本和性能并非呈线性关系，大部分情况下，分布式系统是一个更优的选择。


### 本项目主要特点
- 🔨:实现了基于LRU的缓存淘汰策略
- 📞:支持HTTP，rpc通信协议
- ⏰:使用锁机制和哈希表标记key的方法，防止缓存击穿
- 🎯:使用一致性哈希选择节点，实现负载均衡
- ☁ :使用protobuf和grpc来进行结点间的通信，二进制通信并http文本传输效率更快

#### 核心数据结构Group：负责与用户的交互，并且控制缓存值存储和获取的流程:

```
                    是
接收 key --> 检查是否被缓存 -----> 返回缓存值 ⑴
                |  否                         是
                |-----> 是否应当从远程节点获取 -----> 与远程节点交互 --> 返回缓存值 ⑵
                            |  否
                            |-----> 调用`回调函数`，获取值并添加到缓存 --> 返回缓存值 ⑶
```
GeeCache 的代码结构:

```
geecache/
    |--lru/
        |--lru.go  // lru 缓存淘汰策略
    |--byteview.go // 缓存值的抽象与封装
    |--cache.go    // 并发控制
    |--geecache.go // 负责与外部交互，控制缓存存储和获取的主流程
```


### 一次查询key的逻辑

```shell
//每执行一次main函数就是起一个节点服务                 本地用户交互前端连接绑定了一个gee节点，其余节点皆为单纯gee缓存数据节点
// Overall flow char										     requsets			先看local有没有		        local
// gee := createGroup() --------> /api Service : 9999 ---------------------------> gee.Get(key) ------> g.mainCache.Get(key)
// 						|											^					|
// 						|											|		  		    |remote 查看远程节点有没有的逻辑
// 						v											|					v
// 				cache Service : 800x								|			g.peers.PickPeer(key)通过一致性哈希找到这个key应该落在的真正节点地址
// 						|create hash ring & init peerGetter			|					|
// 						|registry peers write in g.peer				|					|p.grpcGetters[p.hashRing(key)]
// 						v											|					|
//			grpcPool.Set(otherAddrs...)								|					v
// 		g.peers = gee.RegisterPeers(grpcPool)						|			g.getFromPeer(peerGetter, key)通过grpc向这个真正节点发送请求
// 						|											|					|
// 						|											|					|
// 						v											|					v
// 		http.ListenAndServe("localhost:800x", httpPool)<------------+--------------peerGetter.Get(key)这个节点查看本地有没有，没有就在这个节点本地加载
// 						|											|
// 						|requsets									|
// 						v											|
// 					p.ServeHttp(w, r)								|
// 						|											|
// 						|url.parse()								|
// 						|--------------------------------------------

```

---
小知识点：
- 缓存雪崩：缓存在同一时刻全部失效，造成瞬时DB请求量大、压力骤增，引起雪崩。缓存雪崩通常因为缓存服务器宕机、缓存的 key 设置了相同的过期时间等引起。
- 缓存击穿：一个存在的key，在缓存过期的一刻，同时有大量的请求，这些请求都会击穿到 DB ，造成瞬时DB请求量大、压力骤增。
- 缓存穿透：查询一个不存在的数据，因为不存在则不会写到缓存中，所以每次都会去请求 DB，如果瞬间流量过大，穿透到 DB，导致宕机。

---
踩坑:  
- 编译protobuf生成grpc,网上有些教程的编译语句已经过时了，注意辨别，实际生产中可以编写一份shell脚本来自动化编译

```shell
protoc -I. --go_out=. --go-grpc_out geecachepb/*.proto
```
---
# 代码逻辑结构详细说明

### 1. LRU缓存淘汰策略部分：

- 核心思路: 1. 定义LRU缓存的结构体Cache，其中包括lru实现需要的双向链表和哈希表，最大容量，当前容量，此外还有一个回调函数，用于删除lru节点时触发的回调函数。

```go
type Cache struct {
	maxBytes int64
	nbytes   int64
	ll       *list.List
	cache    map[string]*list.Element
	// 当OnEvicted不为nil时删除节点时触发这个回调函数
	OnEvicted func(key string, value Value)
}
```

### 2. 对LRU部分进行包装，使其支持并发

- 核心思路：1. 定义一个byte类型切片，使其成为缓存数据的拷贝，成为对外读取的接口，防止元数据被外部程序进行修改。2. 对原始lru套上锁进行包装，解决并发操作lru的问题

```go
//-----------------------------------------------------------------
//byte
type ByteView struct {
	b []byte
}

var _ lru.Value = (*ByteView)(nil)

func (v ByteView) Len() int {
	return len(v.b)
}

func (v ByteView) ByteSlice() []byte {
	return cloneByte(v.b)
}

func (v ByteView) String() string {
	return string(v.b)
}

// 深拷贝一份返回副本
func cloneByte(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}

//-----------------------------------------------------------------
// 使用锁来包装裸的lru结构实现并发
type cache struct {
	mu         sync.Mutex
	lru        *lru.Cache
	cacheBytes int64
}


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

```

### 3. 建立http服务端

- 核心思路： 使用go语言的http标准库搭建服务端，提供了本节点被其他节点访问的能力，服务端通过解析分割传来的路径URL，得到key

```go
func (p *HTTPPOOL) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, p.basePath) {
		panic("HTTPPool serving unexpected path: " + r.URL.Path)
	}
	p.Log("%s %s", r.Method, r.URL.Path)
	// /<basepath>/<groupname>/<key> required
	parts := strings.SplitN(r.URL.Path[len(p.basePath):], "/", 2)
	if len(parts) != 2 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	groupName := parts[0]
	key := parts[1]

	group := GetGroup(groupName)
	if group == nil {
		http.Error(w, "no such group: "+groupName, http.StatusNotFound)
		return
	}

	view, err := group.Get(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Write the value to the response body as a proto message.
	body, err := proto.Marshal(&pb.Response{Value: view.ByteSlice()})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(body)
}

```

### 4. 一致性哈希，将服务节点映射通过哈希算法映射为虚拟节点分散到哈希环上,本项目默认使用crc32.checksumIEEE算法，同时定义了哈希函数接口，可切换其他哈希函数

- 核心思路：一致性哈希算法将 key 映射到 2^32 的空间中，将这个数字首尾相连，形成一个环
  - 计算节点/机器(通常使用节点的名称、编号和 IP 地址)的哈希值，放置在环上。
  - 计算 key 的哈希值，放置在环上，顺时针寻找到的第一个节点，就是应选取的节点/机器。
- 为了防止数据倾斜，我们需要增加虚拟节点来使每个真实节点均摊更均衡的范围

```go
type Map struct {
	hash     Hash           //一致性哈希的哈希算法
	replicas int            //虚拟节点倍数
	keys     []int          // 哈希环
	hashmap  map[int]string //虚拟节点和真实节点的映射关系
}

func NewConsistenthash(replicas int, hash Hash) *Map { //创建一个哈希环实例
	m := &Map{
		hash:     hash,
		replicas: replicas,
		hashmap:  make(map[int]string),
	}

	if m.hash == nil {
		m.hash = crc32.ChecksumIEEE
	}
	return m
}

func (m *Map) Add(keys ...string) { // 真实节点的地址 ip + 端口号
	for _, key := range keys {
		for i := 0; i < m.replicas; i++ { // 对于每一个真实节点的地址，创建对应的虚拟节点的名字， strconv.Itoa(i) + key
			hash := int(m.hash([]byte(strconv.Itoa(i) + key)))
			m.keys = append(m.keys, hash) // 将虚拟节点添加到哈希环上
			m.hashmap[hash] = key         // 添加虚拟节点和真实节点的映射关系，可以通过虚拟节点找到真实节点
		}
	}
	sort.Ints(m.keys)
}

func (m *Map) Get(key string) string { 
	if len(key) == 0 {
		return ""
	}

	hash := int(m.hash([]byte(key)))

	idx := sort.Search(len(m.keys), func(i int) bool {
		return m.keys[i] >= hash
	}) // 在哈希环上顺时针找到第一个大于等于这个哈希值的虚拟节点的下标

	return m.hashmap[m.keys[idx%len(m.keys)]] // 找到真实节点映射
}

func (m *Map) Remove(key string) {
	for i := 0; i < m.replicas; i++ {
		hash := int(m.hash([]byte(strconv.Itoa(i) + key)))
		idx := sort.SearchInts(m.keys, hash)
		m.keys = append(m.keys[:idx], m.keys[idx+1:]...)
		delete(m.hashmap, hash)
	}
}
```

### 5. 分布式节点，http方式实现客服端版本

- 核心思路：新增两个接口peerpicker与peerGetter，用之前实现的httppool实现这两个接口的方法，前者包装了一致性哈希的get方法，找到key对应的节点。后者与远程节点实现通信，解析报文，得到对应的结果。

修改http结构体，新增peers为一致性哈希，初始化时使用set将节点映射进去，同时使用httpgetter封装远程节点与地址与请求体url（包含了一个/_geecache/路径）的映射

```go
type HTTPPool struct {
	// this peer's base URL, e.g. "https://example.net:8000"
	self        string
	basePath    string
	mu          sync.Mutex // guards peers and httpGetters
	peers       *consistenthash.Map
	httpGetters map[string]*httpGetter // keyed by e.g. "http://10.0.0.2:8008"
}
```

抽象接口定义：前者包装了一致性哈希的get方法，找到key对应的节点。后者与远程节点实现通信，解析报文，得到对应的结果。

```go
type PeerPicker interface {
	PickPeer(key string) (peer PeerGetter, ok bool)
}

// PeerGetter is the interface that must be implemented by a peer.
type PeerGetter interface {
	Get(group string, key string) ([]byte, error)
}

```

httppool实现接口：  
- Set方法包装了一致性哈希的add节点方法，将真实节点映射到哈希环上，并将其地址包装以下存入httpgetter
- PickPeer方法通过调用哈希算法中的get得到key所在真实节点的地址，返回这个地址被getter包装后的地址

```go

// Set updates the pool's list of peers.
func (p *HTTPPool) Set(peers ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.peers = consistenthash.New(defaultReplicas, nil)
	p.peers.Add(peers...)
	p.httpGetters = make(map[string]*httpGetter, len(peers))
	for _, peer := range peers {
		p.httpGetters[peer] = &httpGetter{baseURL: peer + p.basePath}
	}
}

// PickPeer picks a peer according to key
func (p *HTTPPool) PickPeer(key string) (PeerGetter, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if peer := p.peers.Get(key); peer != "" && peer != p.self {
		p.Log("Pick peer %s", peer)
		return p.httpGetters[peer], true
	}
	return nil, false
}

var _ PeerPicker = (*HTTPPool)(nil)
```

从远端获取结果：picker得到真实地址悲壮包装后的真实地址 + group名字（创建group时起的数据库名字） + key，通过http协议发送请求得到结果，被getfrompeer方法调用

```go
func (h *httpGetter) Get(group string, key string) ([]byte, error) {
	u := fmt.Sprintf(
		"%v%v/%v",
		h.baseURL,
		url.QueryEscape(group),
		url.QueryEscape(key),
	)
	res, err := http.Get(u)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned: %v", res.Status)
	}

	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %v", err)
	}

	return bytes, nil
}

```

主流程：注册节点，支持远程的load key方法， 从远程节点节点获取val

```go
// A Group is a cache namespace and associated data loaded spread over
type Group struct {
	name      string
	getter    Getter
	mainCache cache
	peers     PeerPicker
}

// RegisterPeers registers a PeerPicker for choosing remote peer
func (g *Group) RegisterPeers(peers PeerPicker) {
	if g.peers != nil {
		panic("RegisterPeerPicker called more than once")
	}
	g.peers = peers
}

func (g *Group) load(key string) (value ByteView, err error) {
	if g.peers != nil {
		if peer, ok := g.peers.PickPeer(key); ok {
			if value, err = g.getFromPeer(peer, key); err == nil {
				return value, nil
			}
			log.Println("[GeeCache] Failed to get from peer", err)
		}
	}

	return g.getLocally(key)
}

func (g *Group) getFromPeer(peer PeerGetter, key string) (ByteView, error) {
	bytes, err := peer.Get(g.name, key)
	if err != nil {
		return ByteView{}, err
	}
	return ByteView{b: bytes}, nil
}

```


### 6. 防止缓存击穿

- 核心思路：用锁和哈希表记录当前正在处理的key，使load函数只执行一次

```go
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
```

### 7. 使用protobuf与grpc通信

- 整体思路和http的相同，需要注意的是使用grpc的格式等。
- 定义了protobuf中两个字段，request中包括我们需要得到的group和key，response是我们需要的value
- 流程：前端ip收到查询请求->查看本地节点缓存（geecache.get()）-> 没有的华进入load->进入g.peers.Pickpeer查询该key应该落在哪个真实节点，并得到该节点ip：port->传入getFromPeer进入远程查询逻辑->按照定义的proto写好数据库group名字与查询的key的Request与用于接受响应的response，使用节点的Get与远程节点通信->进入grpc之间通讯的逻辑，Dial建立连接，并建立一个客户端用于这个链接，客户端使用我们自定义的Get方法将Request传入得到结果-> grpc节点收到请求，
- server与client的基本处理格式

```go
//server端：
//1.将grpc绑定一个ip地址，把业务起来
func (p *GrpcPool) Run() {
	lis, err := net.Listen("tcp", "127.0.0.1"+p.self)
	if err != nil {
		panic(err)
	}

	server := grpc.NewServer()
	pb.RegisterGroupCacheServer(server, p)

	reflection.Register(server) // 使用curl调试必须使用反射
	err = server.Serve(lis)
	if err != nil {
		panic(err)
	}
}
//2.实现我们在proto中定义的接口函数， 用于客户端和服务端通信
service GroupCache {
    rpc Get(Request) returns (Response);
}

func (p *GrpcPool) Get(ctx context.Context, in *pb.Request) (*pb.Response, error) {
	p.Log("%s %s", in.Group, in.Key)
	response := &pb.Response{}

	group := GetGroup(in.Group)
	if group == nil {
		p.Log("no such group %v", in.Group)
		return response, fmt.Errorf("no such group %v", in.Group)
	}
	value, err := group.Get(in.Key)
	if err != nil {
		p.Log("get key %v error %v", in.Key, err)
		return response, err
	}

	response.Value = value.ByteSlice()
	return response, nil
}


// 3.实现client端：
// 建立连接，调取方法返回给上层
func (p *GrpcPool) Get(ctx context.Context, in *pb.Request) (*pb.Response, error) {
	p.Log("%s %s", in.Group, in.Key)
	response := &pb.Response{}

	group := GetGroup(in.Group)
	if group == nil {
		p.Log("no such group %v", in.Group)
		return response, fmt.Errorf("no such group %v", in.Group)
	}
	value, err := group.Get(in.Key)
	if err != nil {
		p.Log("get key %v error %v", in.Key, err)
		return response, err
	}

	response.Value = value.ByteSlice()
	return response, nil
}

// 上层逻辑：
// 从远程节点获取key的缓存
func (g *Group) getFromPeer(peer PeerGetter, key string) (ByteView, error) {
	req := &pb.Request{
		Group: g.name,
		Key:   key,
	}
	res := &pb.Response{}
	err := peer.Get(req, res)
	if err != nil {
		return ByteView{}, err
	}
	return ByteView{b: res.Value}, nil
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
```

