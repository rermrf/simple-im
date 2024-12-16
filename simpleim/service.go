package simpleim

import (
	"context"
	"encoding/json"
	"github.com/IBM/sarama"
	"log"
	"strconv"
)

// IMService 代表了后端服务
type IMService struct {
	producer sarama.SyncProducer
}

func (s *IMService) Receive(ctx context.Context, sender int64, msg Message) error {
	// 转发到 Kafka 里面，以通知别的网关节点

	// 审核，如果不通过，就拒绝，也在这个地方做，一定是同步的，而且最先执行

	// 1.先找到接收者
	receivers := s.findMembers()
	// 如果需要同步数据给搜索，在这里做，也可以借助消费 eventName 来
	// 消息记录存储，也在这里做，一般只存一条

	// 2. 通知
	for _, receiver := range receivers {
		if sender == receiver {
			// 本人就不需要转发了
			// 如果有多端，还得转发
			continue
		}
		// 一个个转发
		val, _ := json.Marshal(Event{
			Receiver: receiver,
			Msg:      msg,
		})
		// 注意：
		// 这里要考虑顺序问题了
		// 可以改批量接口
		_, _, err := s.producer.SendMessage(&sarama.ProducerMessage{
			Topic: eventName,
			// 可以考虑在初始化 producer 的时候，使用哈希类的 partition 选取策略
			Key:   sarama.StringEncoder(strconv.FormatInt(receiver, 10)),
			Value: sarama.ByteEncoder(val),
		})
		if err != nil {
			// 引入重试
			log.Println("发送消息失败", err)
			continue
		}
	}
	return nil
}

// 这里模拟根据 cid，也就是聊天 ID 来查找参与了该聊天的成员
func (s *IMService) findMembers() []int64 {
	return []int64{1, 2, 3, 4}
}
