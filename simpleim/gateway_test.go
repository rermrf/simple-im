package simpleim

import (
	"github.com/IBM/sarama"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"sync"
	"testing"
)

type GatewaySuite struct {
	suite.Suite
	client sarama.Client
}

func (s *GatewaySuite) SetupTest() {
	cfg := sarama.NewConfig()
	cfg.Producer.Return.Successes = true
	cfg.Producer.Return.Errors = true
	client, err := sarama.NewClient([]string{"localhost:9094"}, cfg)
	require.NoError(s.T(), err)
	s.client = client
}

func (s *GatewaySuite) TestGateway() {
	// 启动三个实例，分别监听端口 8081、8082和8083，模拟分布式环境
	go func() {
		err := s.startGateway("gateway_8081", ":8081")
		s.T().Log("8081 退出服务", err)
	}()

	go func() {
		err := s.startGateway("gateway_8082", ":8082")
		s.T().Log("8082 退出服务", err)
	}()

	err := s.startGateway("gateway_8083", ":8083")
	s.T().Log("8083 退出服务", err)
}

func (s *GatewaySuite) startGateway(instance, addr string) error {
	// 启动一个 gateway 的实例
	producer, err := sarama.NewSyncProducerFromClient(s.client)
	require.NoError(s.T(), err)
	gateway := &WsGateway{
		conns: &sync.Map{},
		svc: &IMService{
			producer: producer,
		},
		upgrader:   &websocket.Upgrader{},
		client:     s.client,
		instanceId: instance,
	}
	return gateway.Start(addr)
}

func TestWsGateway(t *testing.T) {
	suite.Run(t, new(GatewaySuite))
}
