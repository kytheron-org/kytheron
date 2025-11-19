package kytheron

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	pb "github.com/kytheron-org/kytheron-plugin-go/plugin"
	"github.com/kytheron-org/kytheron/config"
	"github.com/kytheron-org/kytheron/registry"
	"go.uber.org/zap"
	"io"
	"time"
)

var (
	ParsedTopic = "parsed"
	IngestTopic = "ingest"
)

// Processor is going to handle a few things in one place, for now
// - listen to ingest, pass messages to parser
// - take parser response and emit to parsed topic
// - take message from parsed topic
//   - emit to loki for long term storage
//   - run policy evaluation on the log message

type Processor struct {
	config         *config.Config
	registry       *registry.PluginRegistry
	logger         *zap.Logger
	parsedProducer *kafka.Producer
}

func (p *Processor) handleIngestMessage(msg *kafka.Message) error {
	p.logger.Info("message on ingest", zap.String("partition", msg.TopicPartition.String()))
	client, err := p.registry.Parser("cloudtrail")
	if err != nil {
		return err
	}
	// Parse the log
	var log pb.RawLog
	if err := json.Unmarshal(msg.Value, &log); err != nil {
		return err
	}

	stream, err := client.ParseLog(context.TODO(), &log)
	if err != nil {
		return err
	}

	for {
		parsedLog, err := stream.Recv()
		if err == io.EOF {
			stream.CloseSend()
			break
		}

		p.logger.Debug("parsed log received")

		if err != nil {
			return err
		}

		content, err := json.Marshal(parsedLog)
		if err != nil {
			return err
		}

		p.logger.Debug("producing message to parsed topic")
		if err := p.parsedProducer.Produce(&kafka.Message{
			TopicPartition: kafka.TopicPartition{Topic: &ParsedTopic, Partition: kafka.PartitionAny},
			Value:          content,
		}, nil); err != nil {
			return err
		}
		p.logger.Debug("produced message to parsed topic")
	}
	return nil
}

func (p *Processor) runSourceConsumer(messages chan<- string) {
	p.logger.Info("starting source consumer")
	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":  p.config.Kafka.Source.Url,
		"group.id":           "kytheron",
		"auto.offset.reset":  "earliest",
		"enable.auto.commit": "true",
	})

	if err != nil {
		panic(err)
	}

	err = c.SubscribeTopics([]string{"ingest"}, nil)

	if err != nil {
		panic(err)
	}

	produce, err := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": p.config.Kafka.Parser.Url})
	if err != nil {
		panic(err)
	}
	defer produce.Close()
	p.parsedProducer = produce

	run := true

	for run {
		msg, err := c.ReadMessage(time.Second)
		if err != nil && !err.(kafka.Error).IsTimeout() {
			// The client will automatically try to recover from all errors.
			// Timeout is not considered an error because it is raised by
			// ReadMessage in absence of messages.
			p.logger.Warn("failed to read message", zap.Error(err), zap.String("topic", msg.TopicPartition.String()))
			continue
		}

		if msg == nil {
			continue
		}

		if err := p.handleIngestMessage(msg); err != nil {
			p.logger.Warn("failed to handle ingest message", zap.Error(err))
		}

	}

	c.Close()
	messages <- fmt.Sprintf("sourceConsumer stopped")
}

func (p *Processor) runParserConsumer(messages chan<- string) {
	p.logger.Info("starting parser consumer")
	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": p.config.Kafka.Parser.Url,
		"group.id":          "kytheron",
		"auto.offset.reset": "latest",
	})

	if err != nil {
		panic(err)
	}

	err = c.SubscribeTopics([]string{"parsed"}, nil)

	if err != nil {
		panic(err)
	}

	run := true

	for run {
		msg, err := c.ReadMessage(time.Second)
		if err != nil && !err.(kafka.Error).IsTimeout() {
			p.logger.Warn("failed to read message", zap.Error(err), zap.String("topic", msg.TopicPartition.String()))
		}
		if err == nil {
			p.logger.Debug("message received", zap.String("topic", msg.TopicPartition.String()))
		} else if !err.(kafka.Error).IsTimeout() {
			// The client will automatically try to recover from all errors.
			// Timeout is not considered an error because it is raised by
			// ReadMessage in absence of messages.
			p.logger.Warn("parseConsumerError", zap.Error(err))
		}
	}

	c.Close()
	messages <- fmt.Sprintf("parserConsumer stopped")
}

func (p *Processor) Run(cfg *config.Config, pluginRegistry *registry.PluginRegistry) error {
	processors := make(chan string, 2)

	p.config = cfg
	p.registry = pluginRegistry

	go p.runSourceConsumer(processors)
	go p.runParserConsumer(processors)

	for i := 1; i <= 2; i++ {
		msg := <-processors
		p.logger.Info("processor finished", zap.String("topic", msg))
	}

	return nil
}
