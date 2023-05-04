/*
 * @Author: zzzzztw
 * @Date: 2023-05-04 14:28:41
 * @LastEditors: Do not edit
 * @LastEditTime: 2023-05-04 14:31:49
 * @FilePath: /Geecache/geecache/peers.go
 */
package geecache

type PeerPicker interface {
	PickPeer(key string) (peer PeerGetter, ok bool) //根据传入的key在哈希环上选择相应的节点PeerGetter
}

type PeerGetter interface {
	Get(group string, key string) ([]byte, error) // 用于对应的group查找缓存值
}
