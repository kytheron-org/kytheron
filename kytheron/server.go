package kytheron

import (
	"encoding/json"
	"fmt"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/google/uuid"
	"github.com/kytheron-org/kytheron-plugin-go/plugin"
	"github.com/kytheron-org/kytheron/config"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

type LogReceiveHandler func(rawLog *plugin.RawLog) error

type GrpcServer struct {
	plugin.UnimplementedSourcePluginServer
	onLogReceiveHandlers []LogReceiveHandler
	logger               *zap.Logger
}

var _ plugin.SourcePluginServer = &GrpcServer{}

func (s *GrpcServer) StreamLogs(stream plugin.SourcePlugin_StreamLogsServer) error {
	for {
		rawLog, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&plugin.Empty{})
		}

		if err != nil {
			return err
		}

		for _, h := range s.onLogReceiveHandlers {
			if err := h(rawLog); err != nil {
				// Do we return an error to the client,
				// or do we simply log / retry the failed message
				return err
			}
		}
	}
}

func (s *GrpcServer) AddLogHandler(handler LogReceiveHandler) {
	s.onLogReceiveHandlers = append(s.onLogReceiveHandlers, handler)
}

func (s *GrpcServer) Start(cfg *config.Config) error {
	p, err := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": cfg.Kafka.Source.Url})
	if err != nil {
		panic(err)
	}
	defer p.Close()

	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", cfg.Server.Grpc.Port))
	if err != nil {
		return err
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan bool, 1)

	grpcServer := grpc.NewServer(
		grpc.MaxSendMsgSize(cfg.Server.Grpc.MaxSendMessageSize),
		grpc.MaxRecvMsgSize(cfg.Server.Grpc.MaxRecvMessageSize),
	)
	s.AddLogHandler(func(a *plugin.RawLog) error {
		// TODO: We should have a topic per configured ingester
		topic := "ingest"

		logId := uuid.Must(uuid.NewUUID())
		a.Id = logId.String()

		content, err := json.Marshal(a)
		if err != nil {
			return err
		}

		s.logger.Debug("producing message", zap.String("topic", topic))
		return p.Produce(&kafka.Message{
			TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
			Value:          content,
		}, nil)
	})

	plugin.RegisterSourcePluginServer(grpcServer, s)

	go func() {
		s.logger.Info("grpc server listening", zap.String("address", lis.Addr().String()))
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	go func() {
		select {
		case sig := <-sigs:
			println("Received signal:", sig)
			done <- true // Signal main goroutine to exit
		}
	}()

	handshake := map[string]interface{}{
		"type": "handshake",
		"addr": lis.Addr().String(),
	}
	contents, err := json.Marshal(handshake)
	// Print our handshake
	fmt.Println(string(contents))
	<-done

	// Wait for message deliveries before shutting down
	//p.Flush(15 * 1000)
	return nil
}
