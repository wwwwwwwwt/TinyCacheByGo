/*
 * @Author: zzzzztw
 * @Date: 2023-05-04 10:55:11
 * @LastEditors: Do not edit
 * @LastEditTime: 2023-05-04 12:51:53
 * @FilePath: /geecache/geecache/http.go
 */
package geecache

import (
	"fmt"
	"log"
	"net/http"
	"strings"
)

const defaultPath = "/_geecache/"

type HTTPPOOL struct {
	self     string // ip地址 + port端口
	basePath string // 前缀路径
}

func NewHTTPPool(s string) *HTTPPOOL {
	return &HTTPPOOL{
		self:     s,
		basePath: defaultPath,
	}
}

func (p *HTTPPOOL) Log(format string, v ...interface{}) {
	log.Printf(("[SERVER %s]%s"), p.self, fmt.Sprintf(format, v...))
}

func (p *HTTPPOOL) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	// 前面路径就不对
	if !strings.HasPrefix(r.URL.Path, p.basePath) {
		panic("HTTPPool serving unexpected path: " + r.URL.Path)
	}

	p.Log("%s %s", r.Method, r.URL.Path)

	parts := strings.SplitN(r.URL.Path[len(p.basePath):], "/", 2)

	if len(parts) != 2 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	groupname := parts[0]
	key := parts[1]

	group := GetGroup(groupname)

	if group == nil {
		http.Error(w, "no such group"+groupname, http.StatusNotFound)
	}

	view, err := group.Get(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(view.ByteSlice())

}
