package simpleim

import (
	"context"
	"demo"
	"encoding/json"
	"github.com/IBM/sarama"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type WsGateway struct {
	// 连接了实例的客户端
	// 这里使用 uid 作为 key
	// 实践中要考虑到不同的设备
	// 那么这个 key 可能一个复合结构，例如 uid + 设备
	conns *sync.Map
	svc   *IMService

	client     sarama.Client
	instanceId string
	upgrader   *websocket.Upgrader
}

// Start 在这个启动的时候，监听 websocket 的请求，然后在转发到后端服务
func (g *WsGateway) Start(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", g.wsHandler)
	// 监听别的节点转发的消息
	err := g.subscribeMsg()
	if err != nil {
		return err
	}
	log.Println("server will worked on", addr)
	return http.ListenAndServe(addr, mux)
}

// subscribeMsg 订阅消息
func (g *WsGateway) subscribeMsg() error {
	// 用 instance Id 作为消费者组
	cg, err := sarama.NewConsumerGroupFromClient(g.instanceId, g.client)
	if err != nil {
		return err
	}
	go func() {
		err := cg.Consume(context.Background(),
			[]string{eventName},
			demo.NewHandler[Event](g.consume))
		if err != nil {
			log.Println("退出监听消息循环", err)
		}
	}()
	return nil
}

func (g *WsGateway) consume(msg *sarama.ConsumerMessage, evt Event) error {
	// 转发
	// 获取到 receiver
	// 多端同步的时候，要考虑哪个设备连上了我
	val, ok := g.conns.Load(evt.Receiver)
	if !ok {
		log.Println("当前节点上没有这个用户，直接返回")
		return nil
	}
	conn, ok := val.(*Conn)
	if !ok {
		return nil
	}
	return conn.Send(evt.Msg)
}

func (g *WsGateway) wsHandler(w http.ResponseWriter, r *http.Request) {
	upgrader := g.upgrader
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		// 升级失败
		w.Write([]byte("升级 ws 失败"))
		return
	}
	c := &Conn{
		Conn: conn,
	}
	// 拿到发送人的 session
	// 模拟从 token 中拿到 uid
	uid := g.Uid(r)
	// 记录下在线的连接
	g.conns.Store(uid, c)
	go func() {
		defer func() {
			g.conns.Delete(uid)
		}()
		for {
			// 在这里监听用户发过来的消息
			// type 一般不需要处理，前端会和你约定好，type的类型
			typ, msgBytes, err := c.ReadMessage()
			//switch err {
			//case context.DeadlineExceeded:
			//	// 超时的话，可以继续
			//	continue
			//case nil:
			//	switch typ {
			//	case websocket.TextMessage:
			//	}
			//default:
			//	// 都是网络出现问题，或者连接出了问题
			//	return
			//}
			if err != nil {
				return
			}

			switch typ {
			case websocket.TextMessage, websocket.BinaryMessage:
				// 应该知道：谁发的？发给谁？内容是什么？
				var msg Message
				err = json.Unmarshal(msgBytes, &msg)
				if err != nil {
					// 前端来的数据格式不对，正常不可能进来这里
					continue
				}
				// 这里开 goroutine 处理的话，可能会因为网络问题，导致消息顺序出错、并且会导致大量的goroutine 产生
				// 搞一个携程池（任务池），控制住携程池的数量
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), time.Second)
					err = g.svc.Receive(ctx, uid, msg)
					cancel()
					if err != nil {
						// 引入重试
						// 告诉前端
						// 前端怎么知道我哪条出错了
						err = c.Send(Message{
							Seq:     msg.Seq,
							Type:    "Result",
							Content: "Failed",
						})
						if err != nil {
							// 引入重试
							// 记录日志
						}
					}
				}()

			case websocket.CloseMessage:
				conn.Close()
			default:

			}
		}
	}()
}

func (g *WsGateway) Uid(r *http.Request) int64 {
	// 拿到 token
	//token := strings.TrimLeft(r.Header.Get("Authorization"), "Bear ")
	// jwt 解析
	//r.Cookie("sess_id")

	uidStr := r.Header.Get("uid")
	uid, _ := strconv.ParseInt(uidStr, 10, 64)
	return uid
}

// Conn 对 websocket 封装一下
type Conn struct {
	*websocket.Conn
}

func (c *Conn) Send(msg Message) error {
	val, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return c.WriteMessage(websocket.TextMessage, val)
}

// Message 和前端约定好的 Message 格式
type Message struct {
	// 这个是后端的Id
	// 前端有时候支持引用的功能，转发功能的时候，会需要这个id
	Id int64
	// 前端的序列号
	// 不要求全局唯一，正常只要当下这个 websocket 唯一就可以
	// 发过来的消息的序列号
	// 用于前后端关联消息
	Seq string
	// 用来标识不同的消息类型
	// 文本消息、视频消息
	// 系统消息（后端往前端发的，跟 IM 本身管理有关的消息）
	// type = "video" => content = url/资源标识符 key
	Type    string
	Content string
	// 聊天 ID，正常来说这里不是记录目标用户 ID
	// 而是记录代表了这个聊天的 ID
	// channel id
	Cid int64

	// 万一每个消息都要校验 token，可以在这里带
	//Token string
}
