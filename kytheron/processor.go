package kytheron

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	pb "github.com/kytheron-org/kytheron-plugin-go/plugin"
	"github.com/kytheron-org/kytheron/config"
	"github.com/kytheron-org/kytheron/registry"
	"io"
	"time"
)

// Processor is going to handle a few things in one place, for now
// - listen to ingest, pass messages to parser
// - take parser response and emit to parsed topic
// - take message from parsed topic
//   - emit to loki for long term storage
//   - run policy evaluation on the log message

type Processor struct {
	config   *config.Config
	registry *registry.PluginRegistry
}

func (p *Processor) runSourceConsumer(messages chan<- string) {
	fmt.Println("Starting source consumer")
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

	run := true

	parsedTopic := "parsed"

	for run {
		msg, err := c.ReadMessage(time.Second)
		if err == nil {
			fmt.Printf("Message on ingest %s: %s\n", msg.TopicPartition, string(msg.Value))
			client, err := p.registry.Parser("cloudtrail")
			if err != nil {
				fmt.Println("failed getting client", err)
				continue
			}
			// Parse the log
			var log pb.RawLog
			if err := json.Unmarshal(msg.Value, &log); err != nil {
				fmt.Println("failed parsing log", err)
				continue
			}
			fmt.Println(string(log.Data))
			stream, err := client.ParseLog(context.TODO(), &log)
			if err != nil {
				fmt.Println("failed opening stream", err)
				continue
			}

			for {
				parsedLog, err := stream.Recv()
				if err == io.EOF {
					fmt.Println("Received ROF, closing stream")
					stream.CloseSend()
					break
				}

				fmt.Println("received parsed log")

				if err != nil {
					fmt.Println(err)
					break
				}

				content, err := json.Marshal(parsedLog)
				if err != nil {
					fmt.Println(err)
					break
				}
				fmt.Println(string(content))
				fmt.Println("Producing message")
				if err := produce.Produce(&kafka.Message{
					TopicPartition: kafka.TopicPartition{Topic: &parsedTopic, Partition: kafka.PartitionAny},
					Value:          content,
				}, nil); err != nil {
					fmt.Println(err)
				}
				fmt.Println("Parsed log put on topic successfully")
			}
		} else if !err.(kafka.Error).IsTimeout() {
			// The client will automatically try to recover from all errors.
			// Timeout is not considered an error because it is raised by
			// ReadMessage in absence of messages.
			fmt.Printf("sourceConsumer error: %v (%v)\n", err, msg)
		}
	}

	c.Close()
	messages <- fmt.Sprintf("sourceConsumer stopped")
}

func (p *Processor) runParserConsumer(messages chan<- string) {
	fmt.Println("Starting parser consumer")
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
		if err == nil {
			fmt.Printf("Message on parsed %s: %s\n", msg.TopicPartition, string(msg.Value))
		} else if !err.(kafka.Error).IsTimeout() {
			// The client will automatically try to recover from all errors.
			// Timeout is not considered an error because it is raised by
			// ReadMessage in absence of messages.
			fmt.Printf("parserConsumer error: %v (%v)\n", err, msg)
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
		fmt.Println(msg)
	}

	return nil
}
