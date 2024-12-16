package demo

import (
	"github.com/gorilla/websocket"
	"net/http"
	"testing"
	"time"
)

func TestServer(t *testing.T) {
	upgrader := websocket.Upgrader{}
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		// 这个就是用来搞升级的，或者说初始化 ws 的
		// conn 代表一个 websocket 连接
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			// 升级失败
			w.Write([]byte(err.Error()))
			return
		}
		ws := &Ws{Conn: conn}
		// 从 websocker 接收数据
		go func() {
			for {
				// typ 是指的 websocket 中的消息类型
				typ, msg, err := conn.ReadMessage()
				// 这个 error 很难处理
				if err != nil {
					// 基本上这里都是代表连接出了问题
					return
				}
				switch typ {
				case websocket.CloseMessage:
					conn.Close()
					return
				default:
					t.Log(typ, string(msg))
				}
			}
		}()
		go func() {
			// 循环写一些消息到前端
			ticker := time.NewTicker(time.Second * 5)
			for now := range ticker.C {
				err := ws.WriteString("hello, " + now.String())
				if err != nil {
					// 也是连接崩了
					return
				}
			}

		}()
	})
	http.ListenAndServe(":8080", nil)
}

// 对 conn 进行封装，可以方便我们的使用
type Ws struct {
	*websocket.Conn
}

func (ws *Ws) WriteString(data string) error {
	return ws.WriteMessage(websocket.TextMessage, []byte(data))
}
