package demo

import (
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"sync"
	"testing"
)

// 集线器/中转站
type Hub struct {
	// 连上了我这个节点的所有的 websocket 的连接
	// key 是客户端的名称
	// 绝大多数情况下，你需要保存连接
	conns *sync.Map
}

func (h *Hub) AddConn(name string, c *websocket.Conn) {
	h.conns.Store(name, c)
	go func() {
		// 准备接收数据
		for {
			// typ 是指的 websocket 中的消息类型
			typ, msg, err := c.ReadMessage()
			// 这个 error 很难处理
			if err != nil {
				// 基本上这里都是代表连接出了问题
				return
			}
			switch typ {
			case websocket.CloseMessage:
				h.conns.Delete(name)
				c.Close()
				return
			default:
				// 要转发
				log.Println("来自客户端", name, typ, string(msg))
				h.conns.Range(func(key, value any) bool {
					if key == name {
						// 自己的，不需要转发了
						return true
					}
					if wsConn, ok := value.(*websocket.Conn); ok {
						log.Println("转发给", key, string(msg))
						err := wsConn.WriteMessage(typ, msg)
						if err != nil {
							log.Println(err)
						}
					}
					return true
				})
			}
		}
	}()
}

func TestHub(t *testing.T) {
	upgrader := websocket.Upgrader{}
	hub := &Hub{conns: &sync.Map{}}
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			// 升级失败
			w.Write([]byte(err.Error()))
			return
		}
		name := r.URL.Query().Get("name")
		hub.AddConn(name, conn)
	})
	http.ListenAndServe(":8080", nil)
}
