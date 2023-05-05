<!--
 * @Author: zzzzztw
 * @Date: 2023-05-02 14:29:18
 * @LastEditors: Do not edit
 * @LastEditTime: 2023-05-05 11:22:32
 * @FilePath: /TinyCacheByGo/README.md
-->
# 基于Go的简易分布式缓存框架🚀

仿照Go 语言的groupcache，进行开发

### 主要特点
- 🔨:实现了基于LRU的缓存淘汰策略
- 📞:支持HTTP通信协议
- ⏰:使用Go锁机制防止缓存击穿
- 🎯:使用一致性哈希选择节点，实现负载均衡
- ☁:使用protobuf优化结点间的二进制通信

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

编译protobuf生成grpc
```shell
protoc -I. --go_out=. --go-grpc_out geecachepb/*.proto

```