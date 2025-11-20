package kytheron

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/google/uuid"
	pb "github.com/kytheron-org/kytheron-plugin-go/plugin"
	"github.com/kytheron-org/kytheron/config"
	"github.com/kytheron-org/kytheron/registry"
	"go.uber.org/zap"
	"io"
	"net/http"
	"strconv"
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

	taskChan chan *pb.ParsedLog
}

func NewProcessor(cfg *config.Config, reg *registry.PluginRegistry, logger *zap.Logger) *Processor {
	return &Processor{
		logger:   logger,
		config:   cfg,
		registry: reg,
		taskChan: make(chan *pb.ParsedLog),
	}
}

func (p *Processor) handleParsedMessage(msg *kafka.Message) error {
	var parsedLog pb.ParsedLog
	if err := json.Unmarshal(msg.Value, &parsedLog); err != nil {
		return err
	}

	p.logger.Debug(string(parsedLog.Data), zap.String("type", "parsed_queue"))

	p.logger.Debug("parsed message decoded", zap.String("log_id", parsedLog.SourceId), zap.String("parsed_log_id", parsedLog.Id))
	// Put the log on the task channel, so that the log sink can handle it
	p.taskChan <- &parsedLog

	p.logger.Debug("submitting for evaluation", zap.String("log_id", parsedLog.SourceId), zap.String("parsed_log_id", parsedLog.Id))

	return nil
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

	p.logger.Debug(string(log.Data), zap.String("type", "raw_log"))

	p.logger.Debug("ingest message decoded", zap.String("log_id", log.Id))

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
		parsedLog.SourceId = log.Id
		parsedLog.Id = uuid.Must(uuid.NewUUID()).String()

		p.logger.Debug("parsed log received", zap.String("parsed_log_id", parsedLog.Id), zap.String("log_id", parsedLog.SourceId))

		if err != nil {
			return err
		}

		content, err := json.Marshal(parsedLog)
		if err != nil {
			return err
		}

		p.logger.Debug("producing message to parsed topic", zap.String("parsed_log_id", parsedLog.Id))
		if err := p.parsedProducer.Produce(&kafka.Message{
			TopicPartition: kafka.TopicPartition{Topic: &ParsedTopic, Partition: kafka.PartitionAny},
			Value:          content,
		}, nil); err != nil {
			return err
		}
		p.logger.Debug("produced message to parsed topic", zap.String("parsed_log_id", parsedLog.Id))
	}
	return nil
}

func (p *Processor) logSink(messages chan<- string) {

	for {
		select {
		case task := <-p.taskChan:
			// TODO: Support batching these logs to Loki
			p.logger.Debug(string(task.Data), zap.String("type", "task_channel"))

			timeInNano := time.Now().UTC().UnixNano()
			payload := map[string]interface{}{
				"streams": []map[string]interface{}{
					{
						"stream": map[string]interface{}{
							"source_type": task.SourceType,
							"source_name": task.SourceName,
						},
						"values": [][]interface{}{
							{strconv.FormatInt(timeInNano, 10), task.Data, map[string]interface{}{
								"log_id": task.SourceId,
							}},
						},
					},
				},
			}

			payloadJson, err := json.Marshal(payload)
			if err != nil {
				p.logger.Error("failed to marshal payload", zap.Error(err))
				continue
			}

			p.logger.Debug(string(payloadJson))

			resp, err := http.Post(fmt.Sprintf("%s/api/v1/push", p.config.Loki.Url), "application/json", bytes.NewReader(payloadJson))
			if err != nil {
				p.logger.Error("failed to send payload", zap.Error(err))
				continue
			}

			p.logger.Debug("successfully sent payload to Loki", zap.String("response", resp.Status))
		}
	}
	messages <- fmt.Sprintf("log sink closed")
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

		if msg == nil {
			continue
		}

		if err := p.handleParsedMessage(msg); err != nil {
			p.logger.Warn("failed to handle ingest message", zap.Error(err))
		}
		//if err == nil {
		//	p.logger.Debug("message received", zap.String("topic", msg.TopicPartition.String()))
		//} else if !err.(kafka.Error).IsTimeout() {
		//	// The client will automatically try to recover from all errors.
		//	// Timeout is not considered an error because it is raised by
		//	// ReadMessage in absence of messages.
		//	p.logger.Warn("parseConsumerError", zap.Error(err))
		//}
	}

	c.Close()
	messages <- fmt.Sprintf("parserConsumer stopped")
}

func (p *Processor) Run() error {
	processors := make(chan string, 3)

	go p.runSourceConsumer(processors)
	go p.runParserConsumer(processors)
	go p.logSink(processors)

	for i := 1; i <= 3; i++ {
		msg := <-processors
		p.logger.Info("processor finished", zap.String("topic", msg))
	}

	return nil
}
