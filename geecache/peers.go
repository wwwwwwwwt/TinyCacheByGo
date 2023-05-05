/*
 * @Author: zzzzztw
 * @Date: 2023-05-04 14:28:41
 * @LastEditors: Do not edit
 * @LastEditTime: 2023-05-05 11:44:39
 * @FilePath: /TinyCacheByGo/geecache/peers.go
 */
package geecache

import pb "geecache/geecachepb"

type PeerPicker interface {
	PickPeer(key string) (peer PeerGetter, ok bool) //根据传入的key在哈希环上选择相应的节点PeerGetter
}

type PeerGetter interface {
	Get(in *pb.Request, out *pb.Response) error // 用于对应的group查找缓存值
}
