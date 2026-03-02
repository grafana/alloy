package main

import (
	"context"
	"log"
	"time"

	"github.com/IBM/sarama"
)

const (
	topicName     = "test_topic"
	brokerAddress = "kafka:9092"
)

// This app sends and consumes messages via a Kafka topic.

func main() {
	go produceMessages()
	consumeMessages()
}

func produceMessages() {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true

	var producer sarama.SyncProducer
	var err error

	for {
		producer, err = sarama.NewSyncProducer([]string{brokerAddress}, config)
		if err == nil {
			break
		}
		log.Printf("Failed to start Sarama producer: %v, retrying in 5 seconds...", err)
		time.Sleep(5 * time.Second)
	}

	defer producer.Close()
	log.Println("Sarama producer started successfully")

	for {
		message := &sarama.ProducerMessage{
			Topic: topicName,
			Value: sarama.StringEncoder("hello"),
		}

		partition, offset, err := producer.SendMessage(message)
		if err != nil {
			log.Printf("Failed to send message: %v", err)
		} else {
			log.Printf("Message is stored in topic(%s)/partition(%d)/offset(%d)\n", topicName, partition, offset)
		}
		time.Sleep(time.Second)
	}
}

func consumeMessages() {
	config := sarama.NewConfig()
	config.Consumer.Return.Errors = true
	config.Consumer.Offsets.Initial = sarama.OffsetOldest

	var consumerGroup sarama.ConsumerGroup
	var err error

	for {
		consumerGroup, err = sarama.NewConsumerGroup([]string{brokerAddress}, "test_consumer_group", config)
		if err == nil {
			break
		}
		log.Printf("Failed to start Sarama consumer group: %v, retrying in 5 seconds...", err)
		time.Sleep(5 * time.Second)
	}

	defer func() {
		if err := consumerGroup.Close(); err != nil {
			log.Fatalf("Failed to close consumer group: %v", err)
		}
	}()
	log.Println("Sarama consumer group started successfully")

	ctx := context.Background()
	consumer := Consumer{}

	go func() {
		for err := range consumerGroup.Errors() {
			log.Printf("Consumer group error: %v", err)
		}
	}()

	for {
		topics := []string{topicName}
		if err := consumerGroup.Consume(ctx, topics, &consumer); err != nil {
			log.Fatalf("Error from consumer: %v", err)
		}
	}
}

type Consumer struct{}

func (consumer *Consumer) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (consumer *Consumer) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }
func (consumer *Consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for message := range claim.Messages() {
		log.Printf("Message claimed: value = %s, timestamp = %v, topic = %s\n", string(message.Value), message.Timestamp, message.Topic)
		session.MarkMessage(message, "")
	}
	return nil
}
