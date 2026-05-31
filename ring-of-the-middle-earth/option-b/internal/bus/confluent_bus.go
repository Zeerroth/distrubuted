//go:build kafka

// This file is compiled ONLY with `-tags kafka` (production / docker). It keeps
// the heavy confluent-kafka-go (cgo + librdkafka) dependency out of the default
// build so `go test ./...` and `make test` run offline with the MemoryBus.
//
// To enable: `go build -tags kafka ./...` after `go get github.com/confluentinc/
// confluent-kafka-go/v2/kafka`. The KafkaBus below satisfies the same Bus
// interface, so no engine/API code changes.
package bus

import (
	"fmt"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

// KafkaBus is the production Bus backed by Confluent Kafka.
type KafkaBus struct {
	producer *kafka.Producer
	brokers  string
	groupID  string
}

// NewKafkaBus connects a producer to the broker list.
func NewKafkaBus(brokers, groupID string) (*KafkaBus, error) {
	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers":  brokers,
		"enable.idempotence": true, // exactly-once GameOver (spec §13)
		"acks":               "all",
	})
	if err != nil {
		return nil, err
	}
	return &KafkaBus{producer: p, brokers: brokers, groupID: groupID}, nil
}

func (b *KafkaBus) Produce(topic, key string, value []byte) error {
	return b.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Key:            []byte(key),
		Value:          value,
	}, nil)
}

func (b *KafkaBus) Subscribe(topics ...string) (<-chan Message, error) {
	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": b.brokers,
		"group.id":          b.groupID, // 3 instances share one group => rebalance (spec §26)
		"auto.offset.reset": "earliest",
	})
	if err != nil {
		return nil, err
	}
	if err := c.SubscribeTopics(topics, nil); err != nil {
		return nil, err
	}
	out := make(chan Message, 256)
	go func() {
		defer close(out)
		for {
			msg, err := c.ReadMessage(-1)
			if err != nil {
				fmt.Printf("kafka read error: %v\n", err)
				continue
			}
			key := ""
			if msg.Key != nil {
				key = string(msg.Key)
			}
			out <- Message{Topic: *msg.TopicPartition.Topic, Key: key, Value: msg.Value}
		}
	}()
	return out, nil
}

func (b *KafkaBus) Close() error {
	b.producer.Close()
	return nil
}
